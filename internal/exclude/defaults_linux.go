//go:build linux

package exclude

import "path/filepath"

// defaultAbsPaths lists absolute directories skipped by the conservative smart
// exclusion on Linux: pseudo-filesystems and volatile system dirs that either
// cannot be meaningfully sized or only contain runtime state. None of these
// hold real user data.
func defaultAbsPaths(home string) []string {
	return []string{
		"/proc",
		"/sys",
		"/dev",
		"/run",
		"/var/run",
		"/var/lock",
	}
}

func defaultBaseNames() []string { return nil }

// cacheAbsPaths adds cache/library dirs when --skip-caches is set.
func cacheAbsPaths(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".cargo"),
		filepath.Join(home, "go", "pkg"),
	}
}

func cacheBaseNames() []string {
	return []string{"node_modules", ".cache", "__pycache__", ".venv"}
}
