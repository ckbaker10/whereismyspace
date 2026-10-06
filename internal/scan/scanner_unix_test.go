//go:build unix

package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanRealHardlink verifies inode dedup against a genuine hardlink using the
// platform metadata provider (no fake injection).
func TestScanRealHardlink(t *testing.T) {
	root := t.TempDir()
	orig := filepath.Join(root, "orig.bin")
	if err := os.WriteFile(orig, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(orig, filepath.Join(root, "hard.bin")); err != nil {
		t.Skipf("hardlinks unsupported here: %v", err)
	}

	// Compare the dedup delta rather than an absolute total, so the assertion
	// is independent of the host filesystem's directory block size.
	deduped, err := Run(Config{Root: root, ApparentSize: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	counted, err := Run(Config{Root: root, ApparentSize: true, CountLinks: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := counted.Root.Total() - deduped.Root.Total(); got != 1000 {
		t.Errorf("hardlink delta = %d, want 1000 (dedup should drop the second link)", got)
	}
}
