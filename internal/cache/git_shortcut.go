package cache

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // git's object ID, not a security boundary
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// The git-clean shortcut serves a cached file without a stat or a content
// hash when git vouches for its content. "Clean against HEAD" is not enough:
// the cache may have been written at another commit (a restored CI cache, a
// checkout or pull between runs) or while the file was dirty (#1521). So
// each entry stores the git blob ID of the content it was computed from, and
// the shortcut hits only when that equals the file's stage-0 index blob and
// git diff reports the worktree unchanged from the index. Anything git
// cannot vouch for (dirty, unmerged, --assume-unchanged, skip-worktree,
// symlinks, submodules) falls back to the stat and content-hash check.

// gitWorktree is the git evidence for one CheckFiles call.
type gitWorktree struct {
	// blobs maps the absolute path of every tracked regular file whose
	// worktree content equals its index entry to that entry's blob ID.
	blobs map[string]string
	// dirtyCount is how many paths differ between worktree and index.
	dirtyCount int
	// hexLen is the repository's object ID length: 40 (SHA-1) or 64 (SHA-256).
	hexLen int
}

// readGitWorktree reads the index and the worktree-vs-index diff for the
// repository containing scanPaths[0]. ok is false outside a git checkout or
// when git is unavailable.
func readGitWorktree(scanPaths []string) (gitWorktree, bool) {
	if len(scanPaths) == 0 {
		return gitWorktree{}, false
	}
	rawRoot := scanPaths[0]
	if abs, err := filepath.Abs(rawRoot); err == nil {
		rawRoot = abs
	}
	root := normalizeAbsPath(rawRoot)
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return gitWorktree{}, false
	}
	ctx := context.Background()
	topBytes, err := exec.CommandContext(ctx, gitBin, "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return gitWorktree{}, false
	}
	rawTop := strings.TrimSpace(string(topBytes))
	if rawTop == "" {
		return gitWorktree{}, false
	}
	top := normalizeAbsPath(rawTop)

	// Both reads are cheap (tens of ms on a large checkout) and independent.
	var (
		wg               sync.WaitGroup
		index, diff      []byte
		indexErr, diffEr error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		// -v tags --assume-unchanged entries in lower case; -s adds mode,
		// blob and stage.
		index, indexErr = exec.CommandContext(ctx, gitBin, "-C", top, "ls-files", "-s", "-v", "-z").Output()
	}()
	go func() {
		defer wg.Done()
		// Worktree vs index, refreshing stat-dirty entries by content.
		diff, diffEr = exec.CommandContext(ctx, gitBin, "-C", top, "diff", "--name-only", "-z", "--no-renames", "--").Output()
	}()
	wg.Wait()
	if indexErr != nil || diffEr != nil {
		return gitWorktree{}, false
	}

	dirty := map[string]bool{}
	for _, rel := range strings.Split(string(diff), "\x00") {
		if rel != "" {
			dirty[rel] = true
		}
	}
	w := gitWorktree{blobs: map[string]string{}, dirtyCount: len(dirty)}
	prefixes := []string{rawTop}
	if top != rawTop {
		prefixes = append(prefixes, top)
	}
	for _, rec := range bytes.Split(index, []byte{0}) {
		rel, blob, ok := parseIndexRecord(string(rec))
		if !ok || dirty[rel] {
			continue
		}
		w.hexLen = len(blob)
		for _, prefix := range prefixes {
			if abs, err := filepath.Abs(filepath.Join(prefix, filepath.FromSlash(rel))); err == nil {
				w.blobs[abs] = blob
			}
		}
	}
	return w, true
}

// parseIndexRecord parses one `git ls-files -s -v` record,
// "<tag> <mode> <blob> <stage>\t<path>", and reports whether git vouches
// for the worktree content of path: tag H (cached, not --assume-unchanged
// or skip-worktree), a regular-file mode, and stage 0 (not unmerged).
func parseIndexRecord(rec string) (rel, blob string, ok bool) {
	meta, rel, found := strings.Cut(rec, "\t")
	if !found || rel == "" {
		return "", "", false
	}
	fields := strings.Fields(meta)
	if len(fields) != 4 || fields[0] != "H" || fields[3] != "0" {
		return "", "", false
	}
	if fields[1] != "100644" && fields[1] != "100755" {
		return "", "", false
	}
	blob = fields[2]
	if len(blob) != sha1.Size*2 && len(blob) != sha256.Size*2 {
		return "", "", false
	}
	return rel, blob, true
}

// gitBlobID returns git's object ID for a blob with content, in the
// repository's hash (hexLen 40 for SHA-1, 64 for SHA-256).
func gitBlobID(content []byte, hexLen int) string {
	header := "blob " + strconv.Itoa(len(content)) + "\x00"
	if hexLen == sha256.Size*2 {
		h := sha256.New()
		h.Write([]byte(header))
		h.Write(content)
		return hex.EncodeToString(h.Sum(nil))
	}
	h := sha1.New() //nolint:gosec // git's object ID, not a security boundary
	h.Write([]byte(header))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}
