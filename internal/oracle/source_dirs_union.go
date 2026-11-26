package oracle

import (
	"path/filepath"
	"slices"
)

// UnionSourceDirs keeps convention-derived roots first, followed by model
// roots, without compiling the same directory twice. Roots are compared by
// SourceDirKey, so a convention root found relative to `.` and the model's
// absolute spelling of it count once (#1659); the first spelling is kept.
func UnionSourceDirs(convention, model []string) []string {
	seen := make(map[string]bool, len(convention)+len(model))
	var out []string
	for _, dir := range append(append([]string(nil), convention...), model...) {
		if dir == "" {
			continue
		}
		key := SourceDirKey(dir)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, dir)
	}
	return out
}

// SourceDirKey is the spelling-independent identity of a source root or
// classpath entry: absolute against the working directory and cleaned.
// Symlinks are not resolved, matching AbsolutePath.
func SourceDirKey(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Clean(AbsolutePath(dir))
}

// SameSourceDirSet reports whether a and b name the same source roots,
// ignoring spelling, order and duplicates: krit-fir compiles every file under
// its roots as one module, so neither is semantic.
func SameSourceDirSet(a, b []string) bool {
	return slices.Equal(sourceDirKeySet(a), sourceDirKeySet(b))
}

// SameClasspath reports whether a and b resolve the same classpath: entries
// are compared by SourceDirKey in order, since the first entry declaring a
// class wins, and a repeated entry adds nothing after its first occurrence.
func SameClasspath(a, b []string) bool {
	return slices.Equal(classpathKeys(a), classpathKeys(b))
}

func sourceDirKeySet(dirs []string) []string {
	keys := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir != "" {
			keys = append(keys, SourceDirKey(dir))
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

func classpathKeys(entries []string) []string {
	seen := make(map[string]bool, len(entries))
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		key := SourceDirKey(entry)
		if entry == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}
