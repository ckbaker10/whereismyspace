package report

import (
	"fmt"
	"io"
	"time"

	"whereismyspace/internal/scan"
)

// Options controls rendering shared across output modes.
type Options struct {
	Top     int   // number of entries in list mode
	Depth   int   // max depth in tree mode
	MinSize int64 // hide entries below this many bytes
	Files   bool  // list largest files instead of directories
}

// Summary writes the header block common to list and tree output: the root, its
// total, any excluded mounts/rules, and error/timing info.
func Summary(w io.Writer, res *scan.Result, crossMounts bool) {
	fmt.Fprintf(w, "Scanned: %s\n", res.Root.Path)
	fmt.Fprintf(w, "Total:   %s (%d files)\n", Bytes(res.Root.Total()), res.Root.Files())
	if res.CachedAt.IsZero() {
		fmt.Fprintf(w, "Elapsed: %s\n", res.Elapsed.Round(1e6))
	} else {
		fmt.Fprintf(w, "Source:  cached index from %s ago (run --refresh for a live scan)\n",
			time.Since(res.CachedAt).Round(time.Second))
	}

	if len(res.ExcludedMounts) > 0 {
		verb := "excluded (other filesystems)"
		if crossMounts {
			verb = "included via --cross-mounts"
		}
		fmt.Fprintf(w, "\nMounted partitions %s:\n", verb)
		for _, m := range res.ExcludedMounts {
			fmt.Fprintf(w, "  - %s\n", m.Path)
		}
	}
	if n := len(res.ExcludedRules); n > 0 {
		fmt.Fprintf(w, "\nSkipped by rules: %d director%s\n", n, plural(n))
	}
	if res.Errors > 0 {
		fmt.Fprintf(w, "\nWarning: %d entr%s skipped (permission denied or unreadable)\n",
			res.Errors, func() string {
				if res.Errors == 1 {
					return "y"
				}
				return "ies"
			}())
	}
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// flatten collects every directory node except the root into a single slice.
func flatten(root *scan.Node) []*scan.Node {
	var out []*scan.Node
	var rec func(n *scan.Node)
	rec = func(n *scan.Node) {
		for _, c := range n.Children {
			out = append(out, c)
			rec(c)
		}
	}
	rec(root)
	return out
}
