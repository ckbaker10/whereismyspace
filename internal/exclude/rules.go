// Package exclude decides which directories a scan should skip. It combines
// built-in, per-OS defaults (pseudo-filesystems and system dirs, plus an opt-in
// set of cache/library dirs) with user-supplied patterns.
package exclude

import (
	"path/filepath"
	"strings"
)

// Ruleset holds compiled exclusion rules. The zero value excludes nothing; use
// New to build one from options.
type Ruleset struct {
	// absPaths are absolute directory paths whose subtree is skipped entirely.
	absPaths map[string]struct{}
	// baseNames are directory basenames skipped wherever they appear.
	baseNames map[string]struct{}
	// globs are filepath.Match patterns tested against the full path.
	globs []string
}

// Options controls which built-in rule sets are enabled and adds user rules.
type Options struct {
	// SmartExclude enables the conservative per-OS defaults (pseudo-fs, system
	// dirs). Disabled by --no-smart-exclude.
	SmartExclude bool
	// SkipCaches additionally enables cache/library directories (node_modules,
	// .cache, ~/go/pkg, ...). Enabled by --skip-caches.
	SkipCaches bool
	// UserPatterns are extra --exclude values: absolute paths, bare basenames,
	// or globs. Classified by shape.
	UserPatterns []string
	// Home is the user's home directory, used to anchor home-relative default
	// rules (e.g. ~/.cache). Empty disables those rules.
	Home string
}

// New compiles a Ruleset from opts.
func New(opts Options) *Ruleset {
	r := &Ruleset{
		absPaths:  make(map[string]struct{}),
		baseNames: make(map[string]struct{}),
	}
	if opts.SmartExclude {
		for _, p := range defaultAbsPaths(opts.Home) {
			r.absPaths[filepath.Clean(p)] = struct{}{}
		}
		for _, n := range defaultBaseNames() {
			r.baseNames[n] = struct{}{}
		}
	}
	if opts.SkipCaches {
		for _, p := range cacheAbsPaths(opts.Home) {
			r.absPaths[filepath.Clean(p)] = struct{}{}
		}
		for _, n := range cacheBaseNames() {
			r.baseNames[n] = struct{}{}
		}
	}
	for _, p := range opts.UserPatterns {
		r.addUserPattern(p)
	}
	return r
}

func (r *Ruleset) addUserPattern(p string) {
	p = strings.TrimSpace(p)
	if p == "" {
		return
	}
	switch {
	case strings.ContainsAny(p, "*?["):
		r.globs = append(r.globs, p)
	case filepath.IsAbs(p):
		r.absPaths[filepath.Clean(p)] = struct{}{}
	default:
		// A bare name matches that basename anywhere.
		r.baseNames[p] = struct{}{}
	}
}

// Excluded reports whether the directory at absPath (with the given basename)
// should be skipped.
func (r *Ruleset) Excluded(absPath, base string) bool {
	if _, ok := r.absPaths[absPath]; ok {
		return true
	}
	if _, ok := r.baseNames[base]; ok {
		return true
	}
	for _, g := range r.globs {
		if ok, _ := filepath.Match(g, absPath); ok {
			return true
		}
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
	}
	return false
}
