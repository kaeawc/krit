package projectroot

import (
	"os"
	"path/filepath"
)

// Find picks the repository root for repo-local Krit state. It starts from the
// first scan path, treats files as their parent directory, and walks upward to
// the nearest project marker. If no marker is found, it returns the resolved
// starting directory so single-directory scans keep their old locality.
func Find(scanPaths []string) string {
	if len(scanPaths) == 0 || scanPaths[0] == "" {
		return ""
	}
	dir := scanPaths[0]
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		dir = filepath.Dir(dir)
	}
	start := dir
	for {
		if isRootMarkerDir(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

func isRootMarkerDir(dir string) bool {
	for _, name := range []string{".git", "krit.yml", ".krit.yml", "settings.gradle", "settings.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
