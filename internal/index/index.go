// Package index persists a completed scan to disk and serves later scans of the
// same or a nested path from that cached tree, avoiding a fresh filesystem walk.
//
// The drill-down workflow it targets: a sysadmin scans a top directory (say /),
// then immediately investigates a subdirectory (say /var). The second run finds
// the fresh index whose root (/) covers the requested path (/var), extracts the
// /var subtree, and renders it instantly.
package index

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"whereismyspace/internal/scan"
)

// formatVersion guards against reading indexes written by an incompatible build.
const formatVersion = 2

// Meta is the on-disk index: a scanned tree plus the context needed to decide
// whether it may be reused.
type Meta struct {
	Version        int
	Root           string
	CreatedAt      time.Time
	Fingerprint    string // scan options that affect totals; must match to reuse
	ExcludedMounts []string
	ExcludedRules  []string
	Errors         int64
	Tree           scan.Snapshot
}

// DefaultDir returns the directory where indexes are stored. It honours
// $WHEREISMYSPACE_CACHE, then the OS user cache dir, then the temp dir.
func DefaultDir() string {
	if d := os.Getenv("WHEREISMYSPACE_CACHE"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "whereismyspace")
}

// fileFor returns the index filename for a given scan root. The root path is
// hashed so arbitrary paths map to safe, fixed-length filenames.
func fileFor(dir, root string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".idx")
}

// Fingerprint hashes the scan options that change reported totals. An index is
// only reused for a query whose options produce the same fingerprint.
func Fingerprint(crossMounts, apparentSize, smartExclude, skipCaches, followSym, countLinks bool, patterns []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "cross=%t;apparent=%t;smart=%t;caches=%t;sym=%t;links=%t;",
		crossMounts, apparentSize, smartExclude, skipCaches, followSym, countLinks)
	sorted := append([]string(nil), patterns...)
	sort.Strings(sorted)
	for _, p := range sorted {
		io.WriteString(h, "ex="+p+";")
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

// Save writes meta to the cache directory, keyed by its root path. The write is
// atomic (temp file + rename) so a crash cannot leave a half-written index.
func Save(dir string, meta Meta) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	meta.Version = formatVersion
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}

	final := fileFor(dir, meta.Root)
	tmp := final + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	if encErr := gob.NewEncoder(gz).Encode(meta); encErr != nil {
		_ = gz.Close()
		_ = f.Close()
		_ = os.Remove(tmp)
		return encErr
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, final)
}

func load(path string) (*Meta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	var m Meta
	if err := gob.NewDecoder(gz).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Lookup finds the closest fresh index that covers queryPath and returns the
// subtree rooted there. It walks queryPath and its ancestors, checking for an
// index at each, and returns the first (deepest, most specific) match whose
// version, fingerprint, and age all qualify. maxAge <= 0 means never expire.
func Lookup(dir, queryPath, fingerprint string, maxAge time.Duration) (*Meta, scan.Snapshot, bool) {
	p := filepath.Clean(queryPath)
	for {
		if m, err := load(fileFor(dir, p)); err == nil {
			if m.Version == formatVersion && m.Fingerprint == fingerprint &&
				(maxAge <= 0 || time.Since(m.CreatedAt) <= maxAge) {
				if sub, ok := m.Tree.Find(queryPath); ok {
					return m, sub, true
				}
			}
		}
		parent := filepath.Dir(p)
		if parent == p {
			return nil, scan.Snapshot{}, false
		}
		p = parent
	}
}
