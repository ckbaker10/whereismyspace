package scan

import "os"

// fileMeta carries the platform-specific facts the scanner needs about a single
// path, extracted from an os.FileInfo. It abstracts over syscall.Stat_t so the
// scanner core stays portable and testable.
type fileMeta struct {
	dev    uint64 // device id the file lives on (for mount-boundary detection)
	ino    uint64 // inode number (for hardlink dedup); 0 if unavailable
	nlink  uint64 // number of hardlinks; <=1 means dedup is unnecessary
	blocks int64  // allocated 512-byte blocks (actual on-disk usage)
}

// metaProvider extracts platform-specific metadata from a FileInfo. It is an
// interface so tests can inject synthetic device ids to simulate mount
// boundaries without needing a real multi-filesystem setup.
type metaProvider interface {
	meta(info os.FileInfo) fileMeta
}
