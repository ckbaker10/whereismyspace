package index

import (
	"testing"
	"time"

	"whereismyspace/internal/scan"
)

// sampleTree returns a snapshot shaped like /data with child /data/logs.
func sampleTree() scan.Snapshot {
	return scan.Snapshot{
		Path: "/data", Name: "data", SubBytes: 3000, SubFiles: 3,
		Children: []scan.Snapshot{
			{Path: "/data/logs", Name: "logs", SelfBytes: 3000, Files: 3},
		},
	}
}

const fp = "test-fingerprint"

func TestSaveAndLookupExactRoot(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Meta{Root: "/data", Fingerprint: fp, Tree: sampleTree()}); err != nil {
		t.Fatal(err)
	}
	meta, sub, ok := Lookup(dir, "/data", fp, time.Hour)
	if !ok {
		t.Fatal("expected a cache hit for the exact root")
	}
	if sub.Path != "/data" {
		t.Errorf("sub.Path = %q, want /data", sub.Path)
	}
	if got := sub.Node().Total(); got != 3000 {
		t.Errorf("total = %d, want 3000", got)
	}
	if meta.Root != "/data" {
		t.Errorf("meta.Root = %q, want /data", meta.Root)
	}
}

func TestLookupSubdirectory(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Meta{Root: "/data", Fingerprint: fp, Tree: sampleTree()}); err != nil {
		t.Fatal(err)
	}
	// A scan of /data/logs should be served from the /data index.
	_, sub, ok := Lookup(dir, "/data/logs", fp, time.Hour)
	if !ok {
		t.Fatal("expected a cache hit for the nested path")
	}
	if sub.Path != "/data/logs" {
		t.Errorf("sub.Path = %q, want /data/logs", sub.Path)
	}
	if got := sub.Node().Total(); got != 3000 {
		t.Errorf("subtree total = %d, want 3000", got)
	}
}

func TestLookupFingerprintMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Meta{Root: "/data", Fingerprint: fp, Tree: sampleTree()}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := Lookup(dir, "/data", "different-fingerprint", time.Hour); ok {
		t.Error("expected a miss when the fingerprint differs")
	}
}

func TestLookupExpired(t *testing.T) {
	dir := t.TempDir()
	old := Meta{Root: "/data", Fingerprint: fp, CreatedAt: time.Now().Add(-2 * time.Hour), Tree: sampleTree()}
	if err := Save(dir, old); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := Lookup(dir, "/data", fp, time.Hour); ok {
		t.Error("expected a miss for an index older than max-age")
	}
	// maxAge <= 0 disables expiry.
	if _, _, ok := Lookup(dir, "/data", fp, 0); !ok {
		t.Error("expected a hit when expiry is disabled")
	}
}

func TestLookupPathNotCovered(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Meta{Root: "/data", Fingerprint: fp, Tree: sampleTree()}); err != nil {
		t.Fatal(err)
	}
	// A sibling outside the cached root has no covering index.
	if _, _, ok := Lookup(dir, "/other", fp, time.Hour); ok {
		t.Error("expected a miss for a path no index covers")
	}
	// A path inside the root but absent from the tree (e.g. an excluded mount).
	if _, _, ok := Lookup(dir, "/data/logs/missing", fp, time.Hour); ok {
		t.Error("expected a miss for a path not present in the cached tree")
	}
}

func TestFingerprintStability(t *testing.T) {
	a := Fingerprint(false, false, true, false, false, false, []string{"x", "y"})
	b := Fingerprint(false, false, true, false, false, false, []string{"y", "x"}) // order-independent
	if a != b {
		t.Error("fingerprint should not depend on exclude-pattern order")
	}
	c := Fingerprint(true, false, true, false, false, false, []string{"x", "y"}) // cross-mounts differs
	if a == c {
		t.Error("fingerprint should change when an option that affects totals changes")
	}
}
