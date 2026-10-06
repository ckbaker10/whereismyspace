//go:build unix

package scan

import (
	"os"
	"syscall"
)

// unixMeta reads metadata from the underlying syscall.Stat_t available on
// Unix-like systems (Linux, macOS, BSD).
type unixMeta struct{}

func defaultMetaProvider() metaProvider { return unixMeta{} }

func (unixMeta) meta(info os.FileInfo) fileMeta {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		// Fall back to apparent size with no device/inode info.
		return fileMeta{blocks: (info.Size() + 511) / 512}
	}
	return fileMeta{
		dev:    uint64(st.Dev),
		ino:    uint64(st.Ino),
		nlink:  uint64(st.Nlink),
		blocks: int64(st.Blocks),
	}
}
