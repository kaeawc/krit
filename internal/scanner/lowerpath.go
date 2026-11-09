package scanner

import (
	"strings"
	"sync"
)

var (
	lowerPathMemo      sync.Map
	lowerSlashPathMemo sync.Map
)

// LowerPath returns strings.ToLower(path). Path rules lowercase the same file path once per node, so the result is memoized.
func LowerPath(path string) string {
	if v, ok := lowerPathMemo.Load(path); ok {
		return v.(string)
	}
	lower := strings.ToLower(path)
	lowerPathMemo.Store(path, lower)
	return lower
}

// LowerSlashPath returns strings.ToLower(strings.ReplaceAll(path, "\\", "/")), memoized like LowerPath.
func LowerSlashPath(path string) string {
	if v, ok := lowerSlashPathMemo.Load(path); ok {
		return v.(string)
	}
	lower := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	lowerSlashPathMemo.Store(path, lower)
	return lower
}
