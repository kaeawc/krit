package cache

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// relocationTempDir is a temp dir whose path holds no rule path marker:
// t.TempDir() is named after the test, and "test" is one.
func relocationTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "krit-reloc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if rootAffectsRuleVerdicts(dir) {
		t.Skipf("temp dir %s contains a rule path marker", dir)
	}
	return dir
}

// seedRelocationRepo writes a two-file repository at root and an analysis
// cache for it under root/.krit/cache, as a scan there would leave it.
func seedRelocationRepo(t *testing.T, root string) (cachePath string, files []string) {
	t.Helper()
	return seedRelocationRepoMarked(t, root, true)
}

// seedRelocationRepoMarked is seedRelocationRepo with the project marker
// (settings.gradle.kts) optional.
func seedRelocationRepoMarked(t *testing.T, root string, marked bool) (cachePath string, files []string) {
	t.Helper()
	if marked {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("rootProject.name = \"repo\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range map[string]string{"app/src/A.kt": "class A\n", "app/src/B.kt": "class B\n"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	c := &Cache{RuleHash: "rh", ScanPaths: []string{root}, Files: map[string]FileEntry{}}
	for _, p := range files {
		cols := scanner.CollectFindings([]scanner.Finding{{File: p, Line: 1, Col: 1, Rule: "R", RuleSet: "s", Severity: "warning", Message: "m"}})
		c.UpdateEntryColumns(p, &cols)
	}
	cachePath = FilePath(DefaultDir(root), []string{root})
	if err := c.Save(cachePath); err != nil {
		t.Fatal(err)
	}
	return cachePath, files
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// #1662: a .krit/cache restored into a checkout at another absolute path
// (another CI agent's workspace) hits for unchanged files, and its findings
// name the new checkout.
func TestAnalysisCacheHitsAfterRelocatingCheckout(t *testing.T) {
	base := relocationTempDir(t)
	rootA, rootB := filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "agent-2", "repo")
	cacheA, _ := seedRelocationRepo(t, rootA)
	_, filesB := seedRelocationRepo(t, rootB) // the same checkout at B ...
	cacheB := FilePath(DefaultDir(rootB), []string{rootB})
	if filepath.Base(cacheA) != filepath.Base(cacheB) {
		t.Fatalf("cache file names differ across roots: %s vs %s", filepath.Base(cacheA), filepath.Base(cacheB))
	}
	copyFile(t, cacheA, cacheB) // ... with agent 1's cache restored

	c := Load(cacheB)
	res := c.CheckFiles(filesB, "rh", rootB)
	if res.TotalCached != len(filesB) {
		t.Fatalf("relocated cache served %d of %d files", res.TotalCached, len(filesB))
	}
	for _, f := range res.CachedColumns.Findings() {
		if !strings.HasPrefix(f.File, rootB) {
			t.Fatalf("a cached finding names %s, not the new checkout", f.File)
		}
	}
}

// Rules apply path heuristics to absolute paths, so a cache must not follow
// a checkout into (or out of) a root where one could match.
func TestAnalysisCacheDiscardedWhenMovedUnderAHeuristicRoot(t *testing.T) {
	base := relocationTempDir(t)
	rootA, rootB := filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "samples", "repo")
	cacheA, _ := seedRelocationRepo(t, rootA)
	seedRelocationRepo(t, rootB) // the same checkout under samples/
	cacheB := FilePath(DefaultDir(rootB), []string{rootB})
	copyFile(t, cacheA, cacheB)
	if c := Load(cacheB); len(c.Files) != 0 {
		t.Fatalf("a cache moved under samples/ kept %d entries", len(c.Files))
	}
}

// Without a project marker, rules read absolute paths (the root is only the
// scanned directory), so the cache keeps absolute paths and a moved copy
// misses instead of serving verdicts computed at the old path.
func TestAnalysisCacheNotRelocatedWithoutProjectMarker(t *testing.T) {
	base := relocationTempDir(t)
	rootA, rootB := filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "agent-2", "repo")
	cacheA, _ := seedRelocationRepoMarked(t, rootA, false)
	_, filesB := seedRelocationRepoMarked(t, rootB, false)
	cacheB := FilePath(DefaultDir(rootB), []string{rootB})
	copyFile(t, cacheA, cacheB)
	if res := Load(cacheB).CheckFiles(filesB, "rh", rootB); res.TotalCached != 0 {
		t.Fatalf("an unmarked checkout's cache served %d files after a move", res.TotalCached)
	}
}

// A cache outside the repository's default directory (--cache-dir) keeps
// absolute paths, as before.
func TestAnalysisCacheElsewhereKeepsAbsolutePaths(t *testing.T) {
	if got := relocationRoot(filepath.Join(t.TempDir(), "custom", "krit.cache")); got != "" {
		t.Fatalf("relocationRoot of a --cache-dir file = %q, want \"\"", got)
	}
}

// Every path segment a built-in rule tests a file path for must hit
// rulePathMarkers, so a root holding it is never relocated. Adding a rule
// with a new path heuristic fails here: add a marker that covers it.
func TestRulePathMarkersCoverRules(t *testing.T) {
	pathLine := regexp.MustCompile(`(?i)path|\.File\b|filename`)
	literal := regexp.MustCompile(`"([-/a-zA-Z0-9_.]{2,40})"`)
	marker := regexp.MustCompile(`^(/|-)[-a-z0-9_.]+(/[-a-z0-9_.]+)*/?$`)
	checked := 0
	// The rules, plus the shared path classifiers they call.
	shared := map[string]bool{
		filepath.Join("..", "scanner", "testpath.go"):      true,
		filepath.Join("..", "fileignore", "docsample.go"):  true,
		filepath.Join("..", "fileignore", "fileignore.go"): true,
	}
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		if !shared[p] && !strings.HasPrefix(p, filepath.Join("..", "rules")+string(filepath.Separator)) {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if !pathLine.MatchString(line) {
				continue
			}
			for _, m := range literal.FindAllStringSubmatch(line, -1) {
				lit := strings.ToLower(m[1])
				// A marker names a directory; a file name (".xml", ".kt") is
				// matched against the end of a path, never a root.
				if !marker.MatchString(lit) || strings.Count(lit, "/") < 1 || filepath.Ext(strings.TrimSuffix(lit, "/")) != "" && !strings.HasSuffix(lit, "/") {
					continue
				}
				if lit == "/" || lit == "//" {
					continue
				}
				checked++
				if !rootAffectsRuleVerdicts("/x/" + strings.Trim(lit, "/") + "/y") {
					t.Errorf("%s: path heuristic segment %q matches no rulePathMarkers entry", p, m[1])
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("found no path heuristic segments: the scan no longer recognizes the rules' heuristics")
	}
}

// Common CI workspace roots must stay relocatable.
func TestCommonCIRootsAreRelocatable(t *testing.T) {
	for _, root := range []string{
		"/home/runner/work/app/app", "/builds/group/app", "/workspaces/app",
		"/var/lib/jenkins/workspace/app", "/Users/runner/work/app/app", `C:\a\app\app`,
	} {
		if rootAffectsRuleVerdicts(root) {
			t.Errorf("%s is treated as a heuristic root", root)
		}
	}
}

// A path under the root is stored repository-relative with forward slashes
// and re-rooted on load; any other path is stored exactly as given, so an
// outside-root Windows path keeps its separators and still hits (#1806).
func TestRelocatePathRoundTrip(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ci1", "app")
	inside := filepath.Join(root, "sub", "Foo.kt")
	if got := relocatePath(root, inside); got != "sub/Foo.kt" {
		t.Fatalf("relocatePath(inside) = %q, want sub/Foo.kt", got)
	}
	moved := filepath.Join(string(filepath.Separator), "ci2", "app@2")
	if got := rerootPath(moved, relocatePath(root, inside)); got != filepath.Join(moved, "sub", "Foo.kt") {
		t.Fatalf("re-rooted inside path = %q", got)
	}
	for _, outside := range []string{
		filepath.Join(string(filepath.Separator), "elsewhere", "lib", "Bar.kt"),
		root + "-sibling" + string(filepath.Separator) + "Baz.kt",
		filepath.Join("relative", "Qux.kt"),
	} {
		if got := relocatePath(root, outside); got != outside {
			t.Errorf("relocatePath(%q) = %q, want it unchanged", outside, got)
		}
		if got := rerootPath(moved, relocatePath(root, outside)); filepath.IsAbs(outside) && got != outside {
			t.Errorf("round trip of %q = %q, want it unchanged", outside, got)
		}
	}
}
