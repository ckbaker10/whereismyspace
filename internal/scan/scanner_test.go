package scan

import (
	"os"
	"path/filepath"
	"testing"

	"whereismyspace/internal/exclude"
)

// fakeMeta assigns device/inode/nlink metadata by file basename so tests can
// simulate mount boundaries and hardlinks without a special filesystem.
type fakeMeta struct {
	dev   map[string]uint64
	ino   map[string]uint64
	nlink map[string]uint64
}

func (f fakeMeta) meta(info os.FileInfo) fileMeta {
	name := info.Name()
	dev := uint64(1)
	if d, ok := f.dev[name]; ok {
		dev = d
	}
	nlink := uint64(1)
	if n, ok := f.nlink[name]; ok {
		nlink = n
	}
	// Report directories as occupying no blocks so tests can assert exact file
	// totals independent of the host filesystem's directory sizes.
	blocks := int64(0)
	if !info.IsDir() {
		blocks = (info.Size() + 511) / 512
	}
	return fileMeta{
		dev:    dev,
		ino:    f.ino[name],
		nlink:  nlink,
		blocks: blocks,
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanExcludesOtherMounts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), 512)

	sub := filepath.Join(root, "sub")
	mustMkdir(t, sub)
	writeFile(t, filepath.Join(sub, "b.txt"), 1024)
	writeFile(t, filepath.Join(sub, "c.txt"), 1536)

	// "mnt" lives on a different (fake) device and must be skipped entirely.
	mnt := filepath.Join(root, "mnt")
	mustMkdir(t, mnt)
	writeFile(t, filepath.Join(mnt, "big.txt"), 999999)

	cfg := Config{
		Root: root,
		meta: fakeMeta{dev: map[string]uint64{"mnt": 2}},
	}
	res, err := Run(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := res.Root.Total(), int64(3072); got != want {
		t.Errorf("total = %d, want %d", got, want)
	}
	if got, want := res.Root.Files(), int64(3); got != want {
		t.Errorf("files = %d, want %d", got, want)
	}
	if len(res.ExcludedMounts) != 1 || filepath.Base(res.ExcludedMounts[0].Path) != "mnt" {
		t.Errorf("excluded mounts = %+v, want [.../mnt]", res.ExcludedMounts)
	}
}

func TestScanCrossMountsIncludesEverything(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), 512)
	mnt := filepath.Join(root, "mnt")
	mustMkdir(t, mnt)
	writeFile(t, filepath.Join(mnt, "big.txt"), 2048)

	cfg := Config{
		Root:        root,
		CrossMounts: true,
		meta:        fakeMeta{dev: map[string]uint64{"mnt": 2}},
	}
	res, err := Run(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := res.Root.Total(), int64(2560); got != want {
		t.Errorf("total = %d, want %d", got, want)
	}
	if len(res.ExcludedMounts) != 0 {
		t.Errorf("expected no excluded mounts, got %+v", res.ExcludedMounts)
	}
}

func TestScanRuleExclusion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.txt"), 512)
	nm := filepath.Join(root, "node_modules")
	mustMkdir(t, nm)
	writeFile(t, filepath.Join(nm, "junk.txt"), 5000)

	rules := exclude.New(exclude.Options{UserPatterns: []string{"node_modules"}})
	cfg := Config{Root: root, Rules: rules, meta: fakeMeta{}}
	res, err := Run(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := res.Root.Total(), int64(512); got != want {
		t.Errorf("total = %d, want %d (node_modules should be skipped)", got, want)
	}
	if len(res.ExcludedRules) != 1 {
		t.Errorf("excluded rules = %+v, want one entry", res.ExcludedRules)
	}
}

func TestScanHardlinkDedupViaMeta(t *testing.T) {
	root := t.TempDir()
	// Two entries the fake provider reports as the same inode with nlink=2.
	writeFile(t, filepath.Join(root, "l1.txt"), 1024)
	writeFile(t, filepath.Join(root, "l2.txt"), 1024)

	fm := fakeMeta{
		ino:   map[string]uint64{"l1.txt": 555, "l2.txt": 555},
		nlink: map[string]uint64{"l1.txt": 2, "l2.txt": 2},
	}

	// Default: dedup on -> counted once.
	res, err := Run(Config{Root: root, meta: fm}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := res.Root.Total(), int64(1024); got != want {
		t.Errorf("dedup total = %d, want %d", got, want)
	}

	// CountLinks: both counted.
	res, err = Run(Config{Root: root, CountLinks: true, meta: fm}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := res.Root.Total(), int64(2048); got != want {
		t.Errorf("count-links total = %d, want %d", got, want)
	}
}

func TestScanNestedTotalsFinalize(t *testing.T) {
	root := t.TempDir()
	// Deep nesting exercises the concurrent walk + finalize aggregation.
	cur := root
	for i := 0; i < 6; i++ {
		cur = filepath.Join(cur, "d")
		mustMkdir(t, cur)
		writeFile(t, filepath.Join(cur, "f.txt"), 1024)
	}
	res, err := Run(Config{Root: root, Workers: 4, meta: fakeMeta{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := res.Root.Total(), int64(6144); got != want {
		t.Errorf("nested total = %d, want %d", got, want)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
}
