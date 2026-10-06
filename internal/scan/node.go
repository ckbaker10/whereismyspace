package scan

import (
	"sort"
	"sync"
	"sync/atomic"
)

// Node represents a directory in the scanned tree. Sizes are accumulated
// bottom-up: SelfBytes is the size of files directly contained in this
// directory, and Bytes (read via Total) is SelfBytes plus the total of every
// descendant directory.
type Node struct {
	Path      string
	Name      string
	selfBytes atomic.Int64 // bytes of files directly in this dir
	subBytes  atomic.Int64 // bytes contributed by child directories
	files     atomic.Int64 // regular files directly in this dir
	subFiles  atomic.Int64 // files contributed by child directories

	mu       sync.Mutex
	Children []*Node
}

func newNode(path, name string) *Node {
	return &Node{Path: path, Name: name}
}

func (n *Node) addSelf(b int64)  { n.selfBytes.Add(b) }
func (n *Node) addFiles(c int64) { n.files.Add(c) }

// finalize computes subtree totals bottom-up. It must be called once, after all
// concurrent walking has finished, so it can safely run single-threaded.
func (n *Node) finalize() {
	var subBytes, subFiles int64
	for _, c := range n.Children {
		c.finalize()
		subBytes += c.Total()
		subFiles += c.Files()
	}
	n.subBytes.Store(subBytes)
	n.subFiles.Store(subFiles)
}

// Total returns the full size of the subtree rooted at n.
func (n *Node) Total() int64 { return n.selfBytes.Load() + n.subBytes.Load() }

// SelfBytes returns the size of files directly contained in this directory.
func (n *Node) SelfBytes() int64 { return n.selfBytes.Load() }

// Files returns the number of regular files counted in the subtree.
func (n *Node) Files() int64 { return n.files.Load() + n.subFiles.Load() }

func (n *Node) addChild(c *Node) {
	n.mu.Lock()
	n.Children = append(n.Children, c)
	n.mu.Unlock()
}

// SortedChildren returns this node's direct children ordered largest first.
func (n *Node) SortedChildren() []*Node {
	n.mu.Lock()
	out := make([]*Node, len(n.Children))
	copy(out, n.Children)
	n.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Total() > out[j].Total() })
	return out
}

// hardlinkSet tracks (device, inode) pairs so a file with multiple hardlinks is
// only counted once. It is safe for concurrent use.
type hardlinkSet struct {
	mu   sync.Mutex
	seen map[linkKey]struct{}
}

type linkKey struct {
	dev uint64
	ino uint64
}

func newHardlinkSet() *hardlinkSet {
	return &hardlinkSet{seen: make(map[linkKey]struct{})}
}

// firstSeen reports whether (dev, ino) is being seen for the first time. It
// returns true the first time a given inode is observed and false thereafter.
func (h *hardlinkSet) firstSeen(dev, ino uint64) bool {
	k := linkKey{dev, ino}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.seen[k]; ok {
		return false
	}
	h.seen[k] = struct{}{}
	return true
}
