//go:build darwin

package exclude

import "path/filepath"

// defaultAbsPaths lists absolute directories skipped by the conservative smart
// exclusion on macOS. /System/Volumes and /Volumes also get caught by the
// mount-boundary check, but excluding them by name keeps output clean when the
// user opts into --cross-mounts.
func defaultAbsPaths(home string) []string {
	return []string{
		"/dev",
		"/System/Volumes",
		"/Volumes",
		"/private/var/vm",
	}
}

func defaultBaseNames() []string { return nil }

func cacheAbsPaths(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Library", "Caches"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".cargo"),
		filepath.Join(home, "go", "pkg"),
	}
}

func cacheBaseNames() []string {
	return []string{"node_modules", ".cache", "__pycache__", ".venv"}
}
