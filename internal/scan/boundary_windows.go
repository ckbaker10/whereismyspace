//go:build windows

package scan

import "os"

// windowsMeta approximates disk usage on Windows. Go's os.FileInfo does not
// expose device/inode numbers portably, so hardlink dedup is unavailable and
// mount-boundary detection relies on the exclusion rules instead. Sizes use the
// apparent file size rounded up to 512-byte blocks.
type windowsMeta struct{}

func defaultMetaProvider() metaProvider { return windowsMeta{} }

func (windowsMeta) meta(info os.FileInfo) fileMeta {
	return fileMeta{blocks: (info.Size() + 511) / 512}
}
