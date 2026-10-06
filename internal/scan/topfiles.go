package scan

import (
	"container/heap"
	"sort"
	"sync"
	"sync/atomic"
)

// FileEntry is a single regular file recorded for the --files listing.
type FileEntry struct {
	Path  string
	Bytes int64
}

// topFiles keeps the K largest files seen, using a min-heap so the smallest of
// the retained files can be evicted in O(log K). An atomic threshold lets the
// common case (a file smaller than everything retained) skip locking entirely.
type topFiles struct {
	k         int
	mu        sync.Mutex
	h         fileMinHeap
	threshold atomic.Int64 // smallest Bytes currently retained once full
}

func newTopFiles(k int) *topFiles {
	if k <= 0 {
		return nil
	}
	return &topFiles{k: k, h: make(fileMinHeap, 0, k)}
}

func (t *topFiles) offer(path string, b int64) {
	if t == nil {
		return
	}
	if len(t.h) >= t.k && b <= t.threshold.Load() {
		return // fast path: cannot make the cut, no lock needed
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.h) < t.k {
		heap.Push(&t.h, FileEntry{Path: path, Bytes: b})
	} else if b > t.h[0].Bytes {
		t.h[0] = FileEntry{Path: path, Bytes: b}
		heap.Fix(&t.h, 0)
	} else {
		return
	}
	if len(t.h) >= t.k {
		t.threshold.Store(t.h[0].Bytes)
	}
}

// sorted returns the retained files ordered largest first.
func (t *topFiles) sorted() []FileEntry {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	out := make([]FileEntry, len(t.h))
	copy(out, t.h)
	t.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out
}

type fileMinHeap []FileEntry

func (h fileMinHeap) Len() int           { return len(h) }
func (h fileMinHeap) Less(i, j int) bool { return h[i].Bytes < h[j].Bytes }
func (h fileMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fileMinHeap) Push(x any)        { *h = append(*h, x.(FileEntry)) }
func (h *fileMinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
