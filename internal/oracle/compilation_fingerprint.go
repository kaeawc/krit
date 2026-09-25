package oracle

import (
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// CompilationSources lists every file krit-fir compiles for sourceDirs: each
// `.kt` file under them, with none of the pruning CollectKtFiles applies.
// Generated sources under build directories and ignored paths are in the
// compilation, so their changes must reach the fingerprint too.
func CompilationSources(sourceDirs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, root := range sourceDirs {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // skip an unreadable entry and keep walking
			}
			if !d.IsDir() && strings.HasSuffix(p, ".kt") && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// CompilationFingerprint identifies the inputs a whole-compilation backend's
// facts depend on: every source path with its content hash, and each
// classpath entry and the backend jar by path, size, and modification time.
// Adding, deleting, or editing a source, or changing the classpath or the jar,
// changes the fingerprint. An unreadable source contributes a marker rather
// than failing, the same way classification skips it, so one bad file cannot
// make every run look like a new compilation.
func CompilationFingerprint(sources, classpath []string, jarPath string) string {
	paths := append([]string(nil), sources...)
	sort.Strings(paths)
	h := hashutil.Hasher().New()
	for _, p := range paths {
		contentHash, err := ContentHash(p)
		if err != nil {
			contentHash = "unreadable"
		}
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(contentHash))
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write([]byte{1})
	for _, entry := range append(append([]string(nil), classpath...), jarPath) {
		_, _ = h.Write([]byte(entry))
		_, _ = h.Write([]byte{0})
		if fi, err := os.Stat(entry); err == nil {
			_, _ = h.Write([]byte(strconv.FormatInt(fi.Size(), 10)))
			_, _ = h.Write([]byte{0})
			_, _ = h.Write([]byte(strconv.FormatInt(fi.ModTime().UnixNano(), 10)))
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// staleCompilationHit returns the path of a hit that was not computed against
// the current compilation, or "" when every hit is current. Crash markers are
// skipped: a run never restamps them, so they would request a refresh
// forever. The lowest stale path is returned so the choice is deterministic.
func staleCompilationHit(hits []*CacheEntry, compilation string) string {
	stale := ""
	for _, h := range hits {
		if h.Crashed {
			continue
		}
		if h.CompilationFingerprint != compilation && (stale == "" || h.FilePath < stale) {
			stale = h.FilePath
		}
	}
	return stale
}
