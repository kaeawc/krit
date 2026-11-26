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

// SamePaths reports whether a and b list the same paths in the same order, ignoring only how each
// is spelled (relative or absolute, unclean). krit-fir shares one compile between the oracle and
// a check only when their source roots and classpaths are equal in order, repeats included
// (AnalysisSession.sharesCompileContext), so this agrees with it.
func SamePaths(a, b []string) bool {
	return slices.EqualFunc(a, b, func(x, y string) bool { return SourceDirKey(x) == SourceDirKey(y) })
}
