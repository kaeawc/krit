package fsutil

import (
	"path/filepath"
	"strings"
)

// RelSlash returns path relative to root with forward slashes. It returns
// path (slash-normalized) when it is not under root or cannot be made relative.
func RelSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	// Match ".." only as a whole leading component: "..generated/A.kt" is inside root.
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// CommonDir returns the deepest directory containing every path in paths
// (each treated as a file or directory path), or "" when paths is empty or
// they share no directory. Fingerprints spell paths relative to it to stay
// the same when a whole checkout moves to another absolute path.
func CommonDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	common := filepath.Dir(filepath.Clean(paths[0]))
	for _, p := range paths[1:] {
		p = filepath.Clean(p)
		for common != "" && !isWithin(common, p) {
			parent := filepath.Dir(common)
			if parent == common {
				common = ""
				break
			}
			common = parent
		}
	}
	return common
}

func isWithin(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
