//go:build windows

package exclude

import "path/filepath"

// defaultAbsPaths on Windows relies mostly on basenames, since system files of
// interest live at well-known names rather than fixed absolute paths across
// drive letters.
func defaultAbsPaths(home string) []string { return nil }

func defaultBaseNames() []string {
	return []string{
		"System Volume Information",
		"$Recycle.Bin",
		"pagefile.sys",
		"hiberfil.sys",
		"swapfile.sys",
	}
}

func cacheAbsPaths(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "AppData", "Local", "Temp"),
		filepath.Join(home, "go", "pkg"),
	}
}

func cacheBaseNames() []string {
	return []string{"node_modules", "__pycache__", ".venv"}
}
