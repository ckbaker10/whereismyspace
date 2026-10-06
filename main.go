// Command whereismyspace scans a directory tree and reports the biggest space
// consumers. By default it stays on a single filesystem (it does not descend
// into other mounted partitions) and skips pseudo/system directories, so
// sysadmins can quickly see what is really filling up a disk.
//
// Author: Lukas Bockel <https://github.com/ckbaker10>
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"whereismyspace/internal/exclude"
	"whereismyspace/internal/index"
	"whereismyspace/internal/report"
	"whereismyspace/internal/scan"
)

var version = "0.1.0"

const (
	author    = "Lukas Bockel"
	authorURL = "https://github.com/ckbaker10"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		top          = flag.Int("top", 20, "number of entries to show in list mode")
		tree         = flag.Bool("tree", false, "render a depth-limited tree instead of a list")
		depth        = flag.Int("depth", 2, "maximum depth in tree mode")
		asJSON       = flag.Bool("json", false, "emit machine-readable JSON")
		crossMounts  = flag.Bool("cross-mounts", false, "descend into other mounted partitions")
		noSmart      = flag.Bool("no-smart-exclude", false, "disable built-in pseudo-fs/system exclusions")
		skipCaches   = flag.Bool("skip-caches", false, "also skip cache/library directories")
		minSizeStr   = flag.String("min-size", "", "hide entries below this size (e.g. 100M, 1.5G)")
		files        = flag.Bool("files", false, "list the largest files instead of directories")
		apparentSize = flag.Bool("apparent-size", false, "report logical file size instead of allocated blocks")
		followSym    = flag.Bool("follow-symlinks", false, "follow symlinks during the walk")
		countLinks   = flag.Bool("count-links", false, "count hardlinked inodes multiple times")
		workers      = flag.Int("workers", 0, "concurrency (default: number of CPUs)")
		noProgress   = flag.Bool("no-progress", false, "disable live progress output")
		noCache      = flag.Bool("no-cache", false, "do not read or write the on-disk index")
		refresh      = flag.Bool("refresh", false, "force a live scan, ignoring any cached index")
		maxAge       = flag.Duration("max-age", time.Hour, "reuse a cached index only if younger than this (0 = never expire)")
		cacheDir     = flag.String("cache-dir", "", "directory for the index (default: OS cache dir)")
		showVersion  = flag.Bool("version", false, "print version and exit")
	)
	var excludes multiFlag
	flag.Var(&excludes, "exclude", "extra path/name/glob to skip (repeatable)")

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("whereismyspace %s\nby %s <%s>\n", version, author, authorURL)
		return 0
	}

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	var minSize int64
	if *minSizeStr != "" {
		minSize, err = report.ParseSize(*minSizeStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	}

	home, _ := os.UserHomeDir()
	rules := exclude.New(exclude.Options{
		SmartExclude: !*noSmart,
		SkipCaches:   *skipCaches,
		UserPatterns: excludes,
		Home:         home,
	})

	trackFiles := 0
	if *files {
		trackFiles = *top
		if trackFiles <= 0 {
			trackFiles = 20
		}
	}

	opts := report.Options{
		Top:     *top,
		Depth:   *depth,
		MinSize: minSize,
		Files:   *files,
	}

	// Cache setup. The fingerprint captures every option that changes reported
	// totals, so an index is only reused for a compatible query.
	cacheEnabled := !*noCache
	dir := *cacheDir
	if dir == "" {
		dir = index.DefaultDir()
	}
	fp := index.Fingerprint(*crossMounts, *apparentSize, !*noSmart, *skipCaches, *followSym, *countLinks, excludes)

	// The --files listing needs per-file data that the index does not retain, so
	// it always requires a live scan.
	canUseCache := cacheEnabled && !*refresh && !*files

	if canUseCache {
		if meta, sub, ok := index.Lookup(dir, absRoot, fp, *maxAge); ok {
			res := resultFromCache(meta, sub, absRoot)
			render(res, opts, *asJSON, *tree)
			return 0
		}
	}

	cfg := scan.Config{
		Root:          absRoot,
		Rules:         rules,
		CrossMounts:   *crossMounts,
		ApparentSize:  *apparentSize,
		FollowSymlink: *followSym,
		CountLinks:    *countLinks,
		Workers:       *workers,
		TrackFiles:    trackFiles,
	}

	progress := makeProgress(!*noProgress && !*asJSON)
	res, err := scan.Run(cfg, progress.update)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	progress.done()

	if cacheEnabled {
		if err := saveIndex(dir, res, fp); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not write index:", err)
		}
	}

	render(res, opts, *asJSON, *tree)
	return 0
}

// render writes a scan result in the selected output mode.
func render(res *scan.Result, opts report.Options, asJSON, tree bool) {
	switch {
	case asJSON:
		if err := report.JSON(os.Stdout, res, opts); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	case tree:
		report.Tree(os.Stdout, res, opts)
	default:
		report.List(os.Stdout, res, opts)
	}
}

// resultFromCache rebuilds a scan.Result from the extracted subtree of a cached
// index, restricting excluded-mount/rule lists to entries under the query path.
func resultFromCache(meta *index.Meta, sub scan.Snapshot, queryPath string) *scan.Result {
	mounts := make([]scan.ExcludedMount, 0)
	for _, p := range meta.ExcludedMounts {
		if underOrEqual(p, queryPath) {
			mounts = append(mounts, scan.ExcludedMount{Path: p})
		}
	}
	var rules []string
	for _, p := range meta.ExcludedRules {
		if underOrEqual(p, queryPath) {
			rules = append(rules, p)
		}
	}
	return &scan.Result{
		Root:           sub.Node(),
		ExcludedMounts: mounts,
		ExcludedRules:  rules,
		CachedAt:       meta.CreatedAt,
	}
}

// saveIndex persists a completed scan as an index keyed by its root.
func saveIndex(dir string, res *scan.Result, fingerprint string) error {
	mounts := make([]string, 0, len(res.ExcludedMounts))
	for _, m := range res.ExcludedMounts {
		mounts = append(mounts, m.Path)
	}
	return index.Save(dir, index.Meta{
		Root:           res.Root.Path,
		CreatedAt:      time.Now(),
		Fingerprint:    fingerprint,
		ExcludedMounts: mounts,
		ExcludedRules:  res.ExcludedRules,
		Errors:         res.Errors,
		Tree:           res.Root.Snapshot(),
	})
}

// underOrEqual reports whether path equals or is nested within base.
func underOrEqual(path, base string) bool {
	path = filepath.Clean(path)
	base = filepath.Clean(base)
	if path == base {
		return true
	}
	if base == string(filepath.Separator) {
		return strings.HasPrefix(path, base)
	}
	return strings.HasPrefix(path, base+string(filepath.Separator))
}

func usage() {
	fmt.Fprintf(os.Stderr, `whereismyspace %s - find what is using disk space
by %s <%s>

Usage:
  whereismyspace [flags] [path]

If no path is given, the current directory is scanned. By default the scan
stays on a single filesystem (other mounted partitions inside subfolders are
excluded) and skips pseudo/system directories.

Each scan is cached as an on-disk index. A later scan of the same path, or of
any subdirectory it covers, is served instantly from that index while it is
fresh (see --max-age). Use --refresh to force a live scan or --no-cache to
disable the index entirely.

Flags:
`, version, author, authorURL)
	flag.PrintDefaults()
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// progressReporter throttles live progress written to stderr. It is a no-op
// when disabled or when stderr is not a terminal.
type progressReporter struct {
	enabled bool
	mu      sync.Mutex
	last    time.Time
	printed bool
}

func makeProgress(want bool) *progressReporter {
	return &progressReporter{enabled: want && isTerminal(os.Stderr)}
}

func (p *progressReporter) update(bytes int64, path string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if now.Sub(p.last) < 100*time.Millisecond {
		return
	}
	p.last = now
	p.printed = true
	line := fmt.Sprintf("\rscanning… %s  %s", report.Bytes(bytes), truncate(path, 60))
	fmt.Fprintf(os.Stderr, "%-100s", line)
}

func (p *progressReporter) done() {
	if !p.enabled || !p.printed {
		return
	}
	fmt.Fprintf(os.Stderr, "\r%-100s\r", "")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
