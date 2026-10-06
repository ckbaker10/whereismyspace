package exclude

import (
	"path/filepath"
	"testing"
)

func TestUserPatternClassification(t *testing.T) {
	r := New(Options{
		UserPatterns: []string{
			"/var/lib/docker", // absolute path
			"node_modules",    // bare basename
			"*.bak",           // glob
		},
	})

	cases := []struct {
		abs  string
		base string
		want bool
	}{
		{"/var/lib/docker", "docker", true},            // exact absolute
		{"/home/u/node_modules", "node_modules", true}, // basename anywhere
		{"/srv/app/node_modules", "node_modules", true},
		{"/data/db.bak", "db.bak", true}, // glob on basename
		{"/var/lib/docker2", "docker2", false},
		{"/home/u/src", "src", false},
	}
	for _, c := range cases {
		if got := r.Excluded(c.abs, c.base); got != c.want {
			t.Errorf("Excluded(%q, %q) = %v, want %v", c.abs, c.base, got, c.want)
		}
	}
}

func TestSmartExcludeToggle(t *testing.T) {
	off := New(Options{SmartExclude: false})
	on := New(Options{SmartExclude: true})

	// Pick a path from the platform defaults, if any, and confirm the toggle
	// controls it. Fall back to a no-op assertion when a platform has no
	// absolute defaults.
	defs := defaultAbsPaths("")
	if len(defs) == 0 {
		return
	}
	p := filepath.Clean(defs[0])
	base := filepath.Base(p)
	if off.Excluded(p, base) {
		t.Errorf("smart-exclude off should not exclude %q", p)
	}
	if !on.Excluded(p, base) {
		t.Errorf("smart-exclude on should exclude %q", p)
	}
}

func TestSkipCachesAddsBasenames(t *testing.T) {
	r := New(Options{SkipCaches: true})
	if !r.Excluded("/anywhere/node_modules", "node_modules") {
		t.Errorf("--skip-caches should exclude node_modules")
	}
}

func TestEmptyRulesetExcludesNothing(t *testing.T) {
	r := New(Options{})
	if r.Excluded("/anything", "anything") {
		t.Errorf("empty ruleset should exclude nothing")
	}
}
