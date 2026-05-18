package projectroot

import (
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/config"
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

// kritRootMarkers are krit-specific signals that a directory is a
// project root, on top of the VCS markers shared via `config.IsVCSRoot`.
var kritRootMarkers = append(append([]string{}, config.Filenames...), "settings.gradle", "settings.gradle.kts")

func isRootMarkerDir(dir string) bool {
	if config.IsVCSRoot(dir) {
		return true
	}
	for _, name := range kritRootMarkers {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
