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
