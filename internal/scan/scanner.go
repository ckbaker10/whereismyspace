// Package scan walks a directory tree in parallel and accumulates disk usage,
// staying on a single filesystem by default and honouring exclusion rules.
package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"whereismyspace/internal/exclude"
)

// Config controls a scan.
type Config struct {
	Root          string           // resolved absolute root path
	Rules         *exclude.Ruleset // directories to skip (may be nil)
	CrossMounts   bool             // descend into other mounted partitions
	ApparentSize  bool             // use logical size instead of allocated blocks
	FollowSymlink bool             // follow symlinks during the walk
	CountLinks    bool             // count hardlinked inodes multiple times
	Workers       int              // parallelism; <=0 means runtime.NumCPU()
	TrackFiles    int              // if >0, retain this many largest files

	meta metaProvider // injectable for tests; nil means platform default
}

// ExcludedMount records a directory that was skipped because it lived on a
// different filesystem than the scan root.
type ExcludedMount struct {
	Path string
}

// Result is the outcome of a scan.
type Result struct {
	Root           *Node
	ExcludedMounts []ExcludedMount
	ExcludedRules  []string    // paths skipped by exclusion rules
	TopFiles       []FileEntry // largest files, if TrackFiles was set
	Errors         int64       // count of unreadable entries skipped
	Elapsed        time.Duration
	CachedAt       time.Time // non-zero when the result was served from a cached index
}

// scanner holds mutable scan state shared across worker goroutines.
type scanner struct {
	cfg     Config
	meta    metaProvider
	sem     chan struct{}
	wg      sync.WaitGroup
	links   *hardlinkSet
	files   *topFiles
	rootDev uint64

	mu             sync.Mutex
	excludedMounts []ExcludedMount
	excludedRules  []string
	errCount       atomic.Int64

	// progress reporting
	bytes    atomic.Int64
	progress func(bytes int64, path string)
}

// Run scans cfg.Root and returns the aggregated tree. progress, if non-nil, is
// called periodically with the running byte total and most recent directory.
func Run(cfg Config, progress func(bytes int64, path string)) (*Result, error) {
	start := time.Now()

	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	cfg.Root = root

	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "scan", Path: root, Err: os.ErrInvalid}
	}

	mp := cfg.meta
	if mp == nil {
		mp = defaultMetaProvider()
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	s := &scanner{
		cfg:      cfg,
		meta:     mp,
		sem:      make(chan struct{}, workers),
		links:    newHardlinkSet(),
		files:    newTopFiles(cfg.TrackFiles),
		rootDev:  mp.meta(info).dev,
		progress: progress,
	}

	rootNode := newNode(root, baseName(root))
	rootNode.addSelf(s.ownBytes(info)) // count the root directory's own blocks
	s.wg.Add(1)
	s.walk(rootNode)
	s.wg.Wait()

	// All workers are done; compute subtree totals single-threaded.
	rootNode.finalize()

	return &Result{
		Root:           rootNode,
		ExcludedMounts: s.excludedMounts,
		ExcludedRules:  s.excludedRules,
		TopFiles:       s.files.sorted(),
		Errors:         s.errCount.Load(),
		Elapsed:        time.Since(start),
	}, nil
}

// walk processes a single directory node. It reads the directory's entries,
// accumulates file sizes into the node, and recurses into subdirectories —
// spawning a new goroutine per subdirectory when a worker slot is free, or
// walking inline otherwise to bound total concurrency. walk consumes exactly
// one wg reference (added by the caller).
func (s *scanner) walk(node *Node) {
	defer s.wg.Done()

	entries, err := os.ReadDir(node.Path)
	if err != nil {
		s.errCount.Add(1)
		return
	}

	var selfBytes, fileCount int64
	for _, de := range entries {
		childPath := filepath.Join(node.Path, de.Name())
		info, err := os.Lstat(childPath)
		if err != nil {
			s.errCount.Add(1)
			continue
		}

		mode := info.Mode()

		// Symlinks: by default do not follow (avoids double-counting and loops),
		// but still count the link's own on-disk size, as du does.
		if mode&os.ModeSymlink != 0 {
			if !s.cfg.FollowSymlink {
				selfBytes += s.ownBytes(info)
				continue
			}
			target, terr := os.Stat(childPath)
			if terr != nil {
				s.errCount.Add(1)
				continue
			}
			info = target
			mode = info.Mode()
		}

		if info.IsDir() {
			s.handleDir(node, childPath, de.Name(), info)
			continue
		}

		if mode.IsRegular() {
			b, counted := s.fileBytes(info)
			if counted {
				selfBytes += b
				fileCount++
				s.files.offer(childPath, b)
			}
		}
		// Non-regular, non-dir entries (devices, sockets, pipes) contribute no
		// meaningful disk usage and are ignored.
	}

	node.addSelf(selfBytes)
	node.addFiles(fileCount)
	if selfBytes != 0 {
		s.bytes.Add(selfBytes)
	}
	if s.progress != nil {
		s.progress(s.bytes.Load(), node.Path)
	}
}

// handleDir decides whether to descend into a subdirectory and, if so, schedules
// the walk. It applies exclusion rules and the mount-boundary check.
func (s *scanner) handleDir(parent *Node, childPath, name string, info os.FileInfo) {
	if s.cfg.Rules != nil && s.cfg.Rules.Excluded(childPath, name) {
		s.mu.Lock()
		s.excludedRules = append(s.excludedRules, childPath)
		s.mu.Unlock()
		return
	}

	if !s.cfg.CrossMounts {
		if dev := s.meta.meta(info).dev; dev != s.rootDev {
			s.mu.Lock()
			s.excludedMounts = append(s.excludedMounts, ExcludedMount{Path: childPath})
			s.mu.Unlock()
			return
		}
	}

	child := newNode(childPath, name)
	child.addSelf(s.ownBytes(info)) // count the directory's own blocks, like du
	parent.addChild(child)

	// Subtree totals are computed later in a single-threaded finalize pass, so
	// here we only need to ensure the child's own walk is scheduled.
	s.wg.Add(1)
	run := func() {
		s.walk(child)
	}

	// Try to grab a worker slot for real parallelism; otherwise walk inline to
	// keep concurrency bounded and avoid unbounded goroutine growth on deep
	// trees.
	select {
	case s.sem <- struct{}{}:
		go func() {
			defer func() { <-s.sem }()
			run()
		}()
	default:
		run()
	}
}

// fileBytes returns the accounted size of a regular file and whether it was
// counted (hardlink dedup may cause it to be skipped).
func (s *scanner) fileBytes(info os.FileInfo) (int64, bool) {
	m := s.meta.meta(info)

	if !s.cfg.CountLinks && m.nlink > 1 && m.ino != 0 {
		if !s.links.firstSeen(m.dev, m.ino) {
			return 0, false
		}
	}

	if s.cfg.ApparentSize {
		return info.Size(), true
	}
	return m.blocks * 512, true
}

// ownBytes returns the on-disk size of an entry's own blocks (used for
// directories and non-followed symlinks), matching how du accounts for them.
func (s *scanner) ownBytes(info os.FileInfo) int64 {
	if s.cfg.ApparentSize {
		return info.Size()
	}
	return s.meta.meta(info).blocks * 512
}

func baseName(p string) string {
	b := filepath.Base(p)
	if b == string(filepath.Separator) || b == "." {
		return p
	}
	return b
}
