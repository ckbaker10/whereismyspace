package report

import (
	"fmt"
	"io"
	"sort"

	"whereismyspace/internal/scan"
)

// List renders the top-N largest directories (or files, when opts.Files is set)
// as a sorted table.
func List(w io.Writer, res *scan.Result, opts Options) {
	Summary(w, res, false)

	if opts.Files {
		listFiles(w, res, opts)
		return
	}

	nodes := flatten(res.Root)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Total() > nodes[j].Total() })

	fmt.Fprintf(w, "\nLargest directories:\n")
	printed := 0
	for _, n := range nodes {
		if opts.MinSize > 0 && n.Total() < opts.MinSize {
			break // sorted descending, nothing smaller qualifies
		}
		if opts.Top > 0 && printed >= opts.Top {
			break
		}
		fmt.Fprintf(w, "  %10s  %s\n", Bytes(n.Total()), n.Path)
		printed++
	}
	if printed == 0 {
		fmt.Fprintf(w, "  (no directories above threshold)\n")
	}
}

func listFiles(w io.Writer, res *scan.Result, opts Options) {
	fmt.Fprintf(w, "\nLargest files:\n")
	printed := 0
	for _, f := range res.TopFiles {
		if opts.MinSize > 0 && f.Bytes < opts.MinSize {
			break
		}
		if opts.Top > 0 && printed >= opts.Top {
			break
		}
		fmt.Fprintf(w, "  %10s  %s\n", Bytes(f.Bytes), f.Path)
		printed++
	}
	if printed == 0 {
		fmt.Fprintf(w, "  (no files above threshold)\n")
	}
}
