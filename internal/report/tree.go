package report

import (
	"fmt"
	"io"

	"whereismyspace/internal/scan"
)

// Tree renders a depth-limited, size-sorted tree of the scan.
func Tree(w io.Writer, res *scan.Result, opts Options) {
	Summary(w, res, false)
	fmt.Fprintf(w, "\n%s  %s\n", Bytes(res.Root.Total()), res.Root.Path)
	printChildren(w, res.Root, opts, "", 1)
}

func printChildren(w io.Writer, node *scan.Node, opts Options, prefix string, depth int) {
	if depth > opts.Depth {
		return
	}
	children := node.SortedChildren()

	// Filter by min-size while preserving order.
	visible := children[:0:0]
	for _, c := range children {
		if opts.MinSize > 0 && c.Total() < opts.MinSize {
			continue
		}
		visible = append(visible, c)
	}

	for i, c := range visible {
		last := i == len(visible)-1
		branch, childPrefix := "├─ ", prefix+"│  "
		if last {
			branch, childPrefix = "└─ ", prefix+"   "
		}
		fmt.Fprintf(w, "%s%s%s  %s\n", prefix, branch, Bytes(c.Total()), c.Name)
		printChildren(w, c, opts, childPrefix, depth+1)
	}
}
