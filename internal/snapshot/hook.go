package snapshot

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
)

//go:embed hook_post_commit.sh
var PostCommitHook string

// HookMarker is the line InstallHook injects after the shebang so
// future InstallHook / UninstallHook calls can recognise their own
// output without overwriting a user's custom hook.
const HookMarker = "# krit-snapshot-hook"

func HookPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".git", "hooks", "post-commit")
}

// InstallHook writes the embedded post-commit hook. With force=false
// it refuses to overwrite an existing non-krit hook so a user's
// hand-rolled hook is preserved.
func InstallHook(repoRoot string, force bool) (string, error) {
	if repoRoot == "" {
		return "", errors.New("snapshot: InstallHook requires repoRoot")
	}
	dir := filepath.Join(repoRoot, ".git", "hooks")
	if _, err := os.Stat(filepath.Dir(dir)); err != nil {
		return "", fmt.Errorf("snapshot: %s is not a git repo: %w", repoRoot, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("snapshot: mkdir %s: %w", dir, err)
	}
	path := HookPath(repoRoot)
	if !force {
		if existing, err := os.ReadFile(path); err == nil && !hasMarker(existing) {
			return "", fmt.Errorf("snapshot: %s exists and is not krit-managed; pass force to overwrite", path)
		}
	}
	if err := fsutil.WriteFileAtomic(path, []byte(taggedHook()), 0o755); err != nil {
		return "", fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	// Best-effort: keep generated snapshot artifacts out of git. Failures
	// here (read-only fs, permission errors) shouldn't block hook install.
	_, _ = EnsureGitignoreEntry(repoRoot, SnapshotsGitignorePattern)
	return path, nil
}

// UninstallHook removes the post-commit hook iff it carries
// HookMarker; a hand-rolled hook is left untouched.
func UninstallHook(repoRoot string) (string, error) {
	if repoRoot == "" {
		return "", errors.New("snapshot: UninstallHook requires repoRoot")
	}
	path := HookPath(repoRoot)
	existing, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		return "", fmt.Errorf("snapshot: read %s: %w", path, err)
	}
	if !hasMarker(existing) {
		return "", fmt.Errorf("snapshot: %s is not krit-managed; refusing to remove", path)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("snapshot: remove %s: %w", path, err)
	}
	return path, nil
}

// taggedHook injects HookMarker right after the shebang so an
// installed hook is detectable on later runs.
func taggedHook() string {
	if shebang, rest, ok := strings.Cut(PostCommitHook, "\n"); ok && strings.HasPrefix(shebang, "#") {
		return shebang + "\n" + HookMarker + "\n" + rest
	}
	return HookMarker + "\n" + PostCommitHook
}

func hasMarker(b []byte) bool {
	return bytes.Contains(b, []byte(HookMarker))
}
