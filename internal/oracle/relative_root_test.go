package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/perf"
)

// chdirProject writes a tiny project and chdirs into it, the way `krit .`
// runs. It returns the (symlink-resolved) project dir.
func chdirProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for rel, src := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		writeTempFile(t, dir, rel, src)
	}
	t.Chdir(dir)
	return dir
}

// krit-types matches --files entries by exact equality with the absolute
// virtual file path, so the miss list handed to the JVM must be absolute even
// when the scan root (and therefore CollectKtFiles) is relative.
func TestPrepareMissTemps_AbsolutizesRelativeMisses(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/main/kotlin/A.kt": "class A\n"})
	srcDir := filepath.Join("src", "main", "kotlin")
	misses, err := CollectKtFiles([]string{srcDir})
	if err != nil || len(misses) != 1 || filepath.IsAbs(misses[0]) {
		t.Fatalf("setup: want one relative miss, got %v (err %v)", misses, err)
	}
	list, fresh, deps, err := prepareMissTemps(misses)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Remove(list)
		_ = os.Remove(fresh)
		_ = os.Remove(deps)
	}()
	data, err := os.ReadFile(list)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "src", "main", "kotlin", "A.kt")
	if got := strings.TrimSpace(string(data)); got != want {
		t.Fatalf("miss list = %q, want absolute %q", got, want)
	}
}

// The JVM reports absolute paths; the Go side keys its cache by the caller's
// (relative) spelling, so the one-shot response must be re-spelled.
func TestOneShotSpelling_RestoresRelativeKeys(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/main/kotlin/A.kt": "class A\n"})
	rel := filepath.Join("src", "main", "kotlin", "A.kt")
	other := filepath.Join("src", "main", "kotlin", "Other.kt")
	sp := oneShotSpelling([]string{filepath.Join("src", "main", "kotlin")}, []string{rel})
	data := &Data{Files: map[string]*File{
		filepath.Join(dir, rel):   {Package: "a"},
		filepath.Join(dir, other): {Package: "o"},
	}}
	sp.CallerData(data)
	if data.Files[rel] == nil || data.Files[other] == nil {
		t.Fatalf("keys not re-spelled relative: %v", fileKeysOf(data.Files))
	}
}

func fileKeysOf(m map[string]*File) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	SetReporter(&diag.Reporter{Warning: &buf})
	t.Cleanup(func() { SetReporter(nil) })
	return &buf
}

func TestWriteSkippedPoisonEntries_ZeroOfNAnalyzedWritesNothing(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A\n")
	b := writeTempFile(t, tmp, "B.kt", "class B\n")
	warnings := captureWarnings(t)

	writeSkippedPoisonEntries(nil, cacheDir, []string{tmp}, []string{a, b}, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, perf.New(false), false)

	hits, misses := ClassifyFiles(cacheDir, []string{a, b})
	if len(hits) != 0 || len(misses) != 2 {
		t.Fatalf("0-of-2 analyzed must not cache markers: hits=%d misses=%d", len(hits), len(misses))
	}
	if !strings.Contains(warnings.String(), "0 of 2") {
		t.Fatalf("expected a warning, got %q", warnings.String())
	}
}

func TestWriteSkippedPoisonEntries_PartialStillMarksSkipped(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A\n")
	b := writeTempFile(t, tmp, "B.kt", "class B\n")
	captureWarnings(t)

	fresh := &Data{Files: map[string]*File{a: {Package: "a"}}}
	writeSkippedPoisonEntries(nil, cacheDir, []string{tmp}, []string{a, b}, fresh, &CacheDepsFile{}, perf.New(false), false)

	hits, _ := ClassifyFiles(cacheDir, []string{b})
	if len(hits) != 1 || !hits[0].Crashed {
		t.Fatalf("unanalyzed file b should carry a skipped marker, got %v", hits)
	}
}

// Markers poisoned by the relative-root bug live in existing caches keyed by
// content hash; they must not be served as empty facts.
func TestClassifyFiles_RelativeSkipPoisonIsMiss(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A\n")
	b := writeTempFile(t, tmp, "B.kt", "class B\n")
	for _, c := range []struct{ path, recorded string }{{a, filepath.Join("src", "A.kt")}, {b, b}} {
		hash, _ := ContentHash(c.path)
		if err := WriteEntry(cacheDir, &CacheEntry{
			ContentHash: hash, FilePath: c.recorded, Crashed: true,
			CrashError: skipPoisonMarker + " file not in Analysis API KtFile set",
		}); err != nil {
			t.Fatal(err)
		}
	}
	hits, misses := ClassifyFiles(cacheDir, []string{a, b})
	if len(misses) != 1 || misses[0] != a {
		t.Fatalf("relative-path poison must be a miss, got misses=%v", misses)
	}
	if len(hits) != 1 || hits[0].FilePath != b {
		t.Fatalf("absolute-path poison remains a hit, got %v", hits)
	}
}
