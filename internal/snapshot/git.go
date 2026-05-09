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
