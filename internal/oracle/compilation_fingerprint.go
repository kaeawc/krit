package oracle

import (
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// CompilationSources lists every file krit-fir compiles for sourceDirs: each
// `.kt` file under them, with none of the pruning CollectKtFiles applies, and
// each `.java` file, which krit-fir hands to K2 as a Java source root so
// Kotlin resolves against it. Generated sources under build directories and
// ignored paths are in the compilation, so their changes must reach the
// fingerprint too; so must a Java edit, which can change a Kotlin verdict
// without changing any Kotlin source.
func CompilationSources(sourceDirs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, root := range sourceDirs {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // skip an unreadable entry and keep walking
			}
			if !d.IsDir() && (strings.HasSuffix(p, ".kt") || strings.HasSuffix(p, ".java")) && !seen[p] {
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
// classpath entry and the backend jar by content (a directory or a missing
// entry by path, plus a directory's size and modification time).
// Adding, deleting, or editing a source, or changing the classpath or the jar,
// changes the fingerprint. An unreadable source contributes a marker rather
// than failing, the same way classification skips it, so one bad file cannot
// make every run look like a new compilation.
func CompilationFingerprint(sources, classpath []string, jarPath string) string {
	return CompilationFingerprintAt("", sources, classpath, jarPath)
}

// CompilationFingerprintAt is CompilationFingerprint with every path spelled
// relative to root when it lies under root (fsutil.RelSlash), so a checkout
// restored at another absolute path, with its jars under another
// GRADLE_USER_HOME, fingerprints the same. root "" keeps absolute paths.
func CompilationFingerprintAt(root string, sources, classpath []string, jarPath string) string {
	key := func(p string) string {
		if root == "" {
			return p
		}
		return fsutil.RelSlash(root, p)
	}
	paths := make([]string, 0, len(sources))
	byKey := make(map[string]string, len(sources))
	for _, p := range sources {
		k := key(p)
		if _, dup := byKey[k]; !dup {
			paths = append(paths, k)
		}
		byKey[k] = p
	}
	sort.Strings(paths)
	h := hashutil.Hasher().New()
	for _, k := range paths {
		contentHash, err := ContentHash(byKey[k])
		if err != nil {
			contentHash = "unreadable"
		}
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(contentHash))
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write([]byte{1})
	for _, entry := range append(append([]string(nil), classpath...), jarPath) {
		id, byContent := classpathEntryIdentity(entry)
		if !byContent {
			_, _ = h.Write([]byte(key(entry)))
		}
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(id))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// classpathEntryIdentity is a regular file's content hash (byContent), or a
// directory's size and modification time, "unreadable", or "missing", which
// only identify the entry together with its path. A file's identity never
// mixes in a stat or a path, so it is the same in every process and at every
// location for the same bytes.
func classpathEntryIdentity(entry string) (id string, byContent bool) {
	fi, err := os.Stat(entry)
	if err != nil {
		return "missing", false
	}
	if fi.IsDir() {
		return strconv.FormatInt(fi.Size(), 10) + "\x00" + strconv.FormatInt(fi.ModTime().UnixNano(), 10), false
	}
	if hx, err := ContentHash(entry); err == nil {
		return hx, true
	}
	return "unreadable", false
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
