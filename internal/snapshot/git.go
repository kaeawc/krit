package snapshot

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// runGitLine runs git inside repoRoot and returns trimmed stdout.
// Wraps stderr into the error so callers don't lose git's diagnostics.
func runGitLine(repoRoot string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = repoRoot
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// ResolveCommitSHA returns `git rev-parse <ref>` inside repoRoot.
// ref defaults to "HEAD".
func ResolveCommitSHA(repoRoot, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	sha, err := runGitLine(repoRoot, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	if len(sha) < 7 {
		return "", fmt.Errorf("git rev-parse %s: unexpected output %q", ref, sha)
	}
	return sha, nil
}

// ResolveParentSHAs returns the parent commit shas for sha. A merge
// returns >1; the initial commit returns nil. Errors propagate to the
// caller; manifest capture swallows them so a missing git stays
// non-fatal.
func ResolveParentSHAs(repoRoot, sha string) ([]string, error) {
	if sha == "" {
		return nil, fmt.Errorf("snapshot: empty sha")
	}
	out, err := runGitLine(repoRoot, "log", "-1", "--format=%P", sha)
	if err != nil {
		return nil, err
	}
	parents := strings.Fields(out)
	if len(parents) == 0 {
		return nil, nil
	}
	return parents, nil
}
