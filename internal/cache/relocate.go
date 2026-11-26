package cache

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/projectroot"
	"github.com/kaeawc/krit/internal/scanner"
)

// The per-file analysis cache in a repository's default cache directory
// (<repo>/.krit/cache) stores every path under the repository relative to
// it, and re-roots them on load, so a .krit/ restored into a checkout at
// another absolute path (a CI cache from another agent) still hits for
// unchanged files (#1662). In memory, paths stay absolute.
//
// Rules see absolute paths (cli/scan absoluteScanPaths) and some test path
// segments ("/samples/", "/src/test/", "-test-utils/", ...). A verdict can
// therefore depend on the checkout root itself when the root contains such
// a segment, so a cache moved from or to such a root is discarded instead.

// rulePathMarkers are the path segments built-in rules test, lowercased, plus
// coarse tokens that cover their unslashed forms ("test" covers "/test/",
// "-testing/", ...). TestRulePathMarkersCoverRules keeps the list in sync
// with internal/rules.
var rulePathMarkers = []string{
	"test", "fixture", "sample", "demo", "playground", "sandbox", "benchmark",
	"tooling", "script", "bazel", "debug", "fake", "mock", "stub", "dummy",
	"smoke", "idling", "instrument", "espresso", "maestro", "canary",
	"generated", "/kapt/", "/ksp/", "/build/", "build-logic", "buildsrc", "/src/",
	"/res/", "/files", "/docs", "/references/", "/examples/", "/database/",
	"snapshot", "/.idea", "/.github", "/.gitlab", "/.circleci", "/.buildkite",
	"/.claude", ".kts",
}

// rootAffectsRuleVerdicts reports whether a rule's path heuristic could
// match inside root.
func rootAffectsRuleVerdicts(root string) bool {
	lower := strings.ToLower(filepath.ToSlash(root)) + "/"
	for _, m := range rulePathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// relocationRoot returns the repository whose default cache directory holds
// cacheFilePath, or "" for a cache anywhere else (--cache-dir, a relocated
// state dir), which keeps absolute paths.
func relocationRoot(cacheFilePath string) string {
	if cacheFilePath == "" {
		return ""
	}
	abs, err := filepath.Abs(cacheFilePath)
	if err != nil {
		return ""
	}
	repo := repoOfCacheDir(filepath.Dir(abs))
	// Rules read paths relative to a marked project root only; under an
	// unmarked one they see absolute paths, so its entries must not move.
	if !projectroot.IsMarked(repo) {
		return ""
	}
	return repo
}

// repoOfCacheDir returns repo when dir is DefaultDir(repo), else "".
func repoOfCacheDir(dir string) string {
	dir = filepath.Clean(dir)
	repo := filepath.Dir(filepath.Dir(dir))
	if DefaultDir(repo) != dir {
		return ""
	}
	return repo
}

// relocatePath spells path for storage under root: repository-relative with
// forward slashes when it lies under root, otherwise unchanged (not
// slash-converted, which would make a Windows path outside the root miss).
func relocatePath(root, path string) string {
	if root == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.ToSlash(rel)
}

// rerootPath inverts relocatePath for a cache loaded at root.
func rerootPath(root, stored string) string {
	if root == "" || stored == "" || filepath.IsAbs(stored) || filepath.VolumeName(stored) != "" {
		return stored
	}
	return filepath.Join(root, filepath.FromSlash(stored))
}

// mapColumnFiles returns columns with every file path passed through fn.
// The file table is copied, so columns itself is not modified.
func mapColumnFiles(columns scanner.FindingColumns, fn func(string) string) scanner.FindingColumns {
	if len(columns.Files) == 0 {
		return columns
	}
	files := make([]string, len(columns.Files))
	for i, f := range columns.Files {
		files[i] = fn(f)
	}
	columns.Files = files
	return columns
}

// relocateLoaded re-roots a cache read from cacheFilePath and reports
// whether it may be used there: a cache moved from or to a root a rule path
// heuristic could match is not.
func relocateLoaded(c *Cache, cacheFilePath string) bool {
	root := relocationRoot(cacheFilePath)
	c.root = root
	saved := c.SavedRoot
	if saved == "" || root == "" {
		return true // absolute paths throughout, as before relocation
	}
	if saved != root && (rootAffectsRuleVerdicts(saved) || rootAffectsRuleVerdicts(root)) {
		return false
	}
	files := make(map[string]FileEntry, len(c.Files))
	for stored, e := range c.Files {
		e.Columns = mapColumnFiles(e.Columns, func(f string) string { return rerootPath(root, f) })
		files[rerootPath(root, stored)] = e
	}
	c.Files = files
	for i, p := range c.ScanPaths {
		c.ScanPaths[i] = rerootPath(root, p)
	}
	return true
}
