package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
)

// SnapshotsGitignorePattern is the entry EnsureGitignoreEntry adds to the
// repo's root .gitignore so users don't accidentally commit captured
// snapshot blobs and manifests.
const SnapshotsGitignorePattern = ".krit/snapshots/"

// EnsureGitignoreEntry appends pattern to the repo root .gitignore if it
// is not already present (as an exact line match, ignoring surrounding
// whitespace). It is best-effort: if the .gitignore can't be read or
// written, the error is returned but callers may choose to ignore it.
//
// Returns added=true when the pattern was newly written, and false if it
// was already present (or if pattern was empty).
func EnsureGitignoreEntry(repoRoot, pattern string) (added bool, err error) {
	if repoRoot == "" {
		return false, errors.New("snapshot: EnsureGitignoreEntry requires repoRoot")
	}
	if pattern == "" {
		return false, nil
	}
	path := filepath.Join(repoRoot, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("snapshot: read %s: %w", path, err)
	}
	if hasGitignoreLine(existing, pattern) {
		return false, nil
	}
	out := append([]byte(nil), existing...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, pattern...)
	out = append(out, '\n')
	if err := fsutil.WriteFileAtomic(path, out, 0o644); err != nil {
		return false, fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	return true, nil
}

// hasGitignoreLine reports whether content contains pattern as a non-comment
// line (trimmed). A negated entry (`!pattern`) does not count as present, since
// it un-ignores the path. Patterns that differ only in trailing slash are
// treated as equivalent so ".krit/snapshots" and ".krit/snapshots/" both match.
func hasGitignoreLine(content []byte, pattern string) bool {
	want := strings.TrimSuffix(pattern, "/")
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		if strings.TrimSuffix(trimmed, "/") == want {
			return true
		}
	}
	return false
}
