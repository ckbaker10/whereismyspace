package scan

import (
	"path/filepath"
	"strings"
)

// Snapshot is a serialisable copy of a directory subtree, safe for gob/JSON
// encoding (unlike Node, which holds atomics and a mutex). Totals are already
// aggregated, as they are after a finalize pass.
type Snapshot struct {
	Path      string
	Name      string
	SelfBytes int64
	SubBytes  int64
	Files     int64
	SubFiles  int64
	Children  []Snapshot
}

// Snapshot converts a finalized Node tree into a serialisable Snapshot.
func (n *Node) Snapshot() Snapshot {
	s := Snapshot{
		Path:      n.Path,
		Name:      n.Name,
		SelfBytes: n.selfBytes.Load(),
		SubBytes:  n.subBytes.Load(),
		Files:     n.files.Load(),
		SubFiles:  n.subFiles.Load(),
	}
	for _, c := range n.Children {
		s.Children = append(s.Children, c.Snapshot())
	}
	return s
}

// Node rebuilds a *Node tree from a Snapshot. The totals are restored directly,
// so no finalize call is needed.
func (s Snapshot) Node() *Node {
	n := newNode(s.Path, s.Name)
	n.selfBytes.Store(s.SelfBytes)
	n.subBytes.Store(s.SubBytes)
	n.files.Store(s.Files)
	n.subFiles.Store(s.SubFiles)
	for _, c := range s.Children {
		n.Children = append(n.Children, c.Node())
	}
	return n
}

// Find returns the snapshot of the subtree rooted at path, if it exists within
// s. Paths are compared after filepath.Clean.
func (s Snapshot) Find(path string) (Snapshot, bool) {
	path = filepath.Clean(path)
	self := filepath.Clean(s.Path)
	if self == path {
		return s, true
	}
	if !underOrEqual(path, self) {
		return Snapshot{}, false
	}
	for _, c := range s.Children {
		if underOrEqual(path, filepath.Clean(c.Path)) {
			return c.Find(path)
		}
	}
	return Snapshot{}, false
}

// underOrEqual reports whether path is equal to or nested within base.
func underOrEqual(path, base string) bool {
	if path == base {
		return true
	}
	if base == string(filepath.Separator) {
		return strings.HasPrefix(path, base)
	}
	return strings.HasPrefix(path, base+string(filepath.Separator))
}
