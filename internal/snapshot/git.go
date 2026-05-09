package snapshot

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ResolveCommitSHA shells out to `git rev-parse <ref>` inside repoRoot.
// ref defaults to "HEAD" when empty. Returns the 40-char sha.
func ResolveCommitSHA(repoRoot, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", ref)
	cmd.Dir = repoRoot
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git rev-parse %s: %s", ref, msg)
	}
	sha := strings.TrimSpace(out.String())
	if len(sha) < 7 {
		return "", fmt.Errorf("git rev-parse %s: unexpected output %q", ref, sha)
	}
	return sha, nil
}

// ResolveParentSHAs returns the list of parent commit shas for sha,
// queried from git inside repoRoot. A merge commit returns >1 parent;
// the initial commit returns an empty slice. When git is unavailable or
// the sha is unknown an error is returned and callers should treat the
// parent list as empty (manifest capture intentionally swallows this).
func ResolveParentSHAs(repoRoot, sha string) ([]string, error) {
	if sha == "" {
		return nil, fmt.Errorf("snapshot: empty sha")
	}
	cmd := exec.CommandContext(context.Background(), "git", "log", "-1", "--format=%P", sha)
	cmd.Dir = repoRoot
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git log %s: %s", sha, msg)
	}
	parents := strings.Fields(strings.TrimSpace(out.String()))
	if len(parents) == 0 {
		return nil, nil
	}
	return parents, nil
}
