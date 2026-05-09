package snapshot

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
)

//go:embed hook_post_commit.sh
var PostCommitHook string

// HookMarker tags krit-installed hooks so InstallHook can refuse to
// overwrite a user's hand-rolled post-commit and so UninstallHook only
// removes hooks that we wrote.
const HookMarker = "# krit-snapshot-hook"

// HookPath returns the canonical post-commit hook path for repoRoot.
func HookPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".git", "hooks", "post-commit")
}

// InstallHook writes the embedded post-commit hook to
// .git/hooks/post-commit. When force is false and an existing hook
// without the krit marker is present, returns an error so the user's
// custom hook is preserved.
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
	if existing, err := os.ReadFile(path); err == nil && !force {
		if !hasMarker(existing) {
			return "", fmt.Errorf("snapshot: %s exists and is not krit-managed; pass force to overwrite", path)
		}
	}
	if err := fsutil.WriteFileAtomic(path, []byte(taggedHook()), 0o755); err != nil {
		return "", fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	return path, nil
}

// UninstallHook removes the post-commit hook iff it was installed by
// krit (carries HookMarker). A user-written hook is left untouched and
// reported via the returned error.
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

// taggedHook returns the embedded script with the marker injected on
// the first non-shebang line so InstallHook can recognise it later.
func taggedHook() string {
	src := PostCommitHook
	if len(src) > 0 && src[0] == '#' {
		// Insert the marker right after the shebang line.
		idx := indexOfNewline(src)
		if idx >= 0 {
			return src[:idx+1] + HookMarker + "\n" + src[idx+1:]
		}
	}
	return HookMarker + "\n" + src
}

func hasMarker(b []byte) bool {
	return contains(splitLines(string(b)), HookMarker)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func indexOfNewline(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}
