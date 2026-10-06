package report

import (
	"encoding/json"
	"io"
	"time"

	"whereismyspace/internal/scan"
)

// jsonNode is the serialised form of a directory subtree.
type jsonNode struct {
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Bytes    int64      `json:"bytes"`
	Files    int64      `json:"files"`
	Children []jsonNode `json:"children,omitempty"`
}

type jsonOutput struct {
	Root           string           `json:"root"`
	TotalBytes     int64            `json:"total_bytes"`
	TotalFiles     int64            `json:"total_files"`
	ElapsedMillis  int64            `json:"elapsed_ms"`
	Cached         bool             `json:"cached"`
	CachedAt       string           `json:"cached_at,omitempty"`
	Errors         int64            `json:"errors"`
	ExcludedMounts []string         `json:"excluded_mounts,omitempty"`
	ExcludedRules  []string         `json:"excluded_rules,omitempty"`
	Tree           jsonNode         `json:"tree"`
	TopFiles       []scan.FileEntry `json:"top_files,omitempty"`
}

// JSON writes the scan result as JSON. The directory tree is emitted down to
// opts.Depth (0 means the full tree); entries below opts.MinSize are omitted.
func JSON(w io.Writer, res *scan.Result, opts Options) error {
	mounts := make([]string, 0, len(res.ExcludedMounts))
	for _, m := range res.ExcludedMounts {
		mounts = append(mounts, m.Path)
	}

	out := jsonOutput{
		Root:           res.Root.Path,
		TotalBytes:     res.Root.Total(),
		TotalFiles:     res.Root.Files(),
		ElapsedMillis:  res.Elapsed.Milliseconds(),
		Cached:         !res.CachedAt.IsZero(),
		Errors:         res.Errors,
		ExcludedMounts: mounts,
		ExcludedRules:  res.ExcludedRules,
		Tree:           buildJSON(res.Root, opts, 0),
		TopFiles:       res.TopFiles,
	}
	if !res.CachedAt.IsZero() {
		out.CachedAt = res.CachedAt.Format(time.RFC3339)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func buildJSON(n *scan.Node, opts Options, depth int) jsonNode {
	jn := jsonNode{
		Path:  n.Path,
		Name:  n.Name,
		Bytes: n.Total(),
		Files: n.Files(),
	}
	if opts.Depth > 0 && depth >= opts.Depth {
		return jn
	}
	for _, c := range n.SortedChildren() {
		if opts.MinSize > 0 && c.Total() < opts.MinSize {
			continue
		}
		jn.Children = append(jn.Children, buildJSON(c, opts, depth+1))
	}
	return jn
}
