package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestBuildManifestData_CarriesForwardCachedPaths is the regression
// pin for issue #254. Under the parse-only-misses warm path, the
// previous implementation only iterated parseResult.{Kotlin,Java}Files
// and wrote a manifest that listed just the dirty files. The next
// analyze re-read that manifest through prepopulatedSourcePaths and
// collapsed the source set to one path — driving the entire daemon
// pipeline into single-file dispatch and dropping ~87 k findings to
// ~62 on $KOTLIN_CORPUS.
//
// The fix: any path present in host.PriorContentHashes that wasn't
// parsed this run (and isn't dirty) gets carried into the new
// manifest using its prior content / structural fingerprint. Cache
// hits keep their entries, deletions still drop out.
func TestBuildManifestData_CarriesForwardCachedPaths(t *testing.T) {
	tmp := t.TempDir()
	parsed := writeManifestTestFile(t, tmp, "Parsed.kt", "package demo\nclass Parsed { fun a() = 1 }\n")
	cached := writeManifestTestFile(t, tmp, "Cached.kt", "package demo\nclass Cached { fun b() = 2 }\n")

	parsedFile := &scanner.File{
		Path:     parsed,
		Language: scanner.LangKotlin,
		Content:  []byte("package demo\nclass Parsed { fun a() = 1 }\n"),
	}

	args := ProjectArgs{
		Paths:       []string{tmp},
		KotlinPaths: []string{parsed, cached},
	}
	host := ProjectHostState{
		FindingsBundleCacheRoot: tmp,
		PriorContentHashes: map[string]string{
			parsed: "stale-hash-recomputed-because-parsed",
			cached: "prior-hash-for-cached",
		},
		PriorStructuralFPs: map[string]string{
			parsed: "stale-fp-recomputed-because-parsed",
			cached: "prior-fp-for-cached",
		},
		SourceSetDirty: []string{parsed},
	}
	parseResult := ParseResult{KotlinFiles: []*scanner.File{parsedFile}}

	m := buildManifestData(args, host, parseResult, scanner.RunFingerprint{}, true)
	if !m.enabled {
		t.Fatalf("manifest disabled: %+v", m)
	}
	if len(m.contentHashes) != 2 {
		t.Errorf("contentHashes len = %d, want 2 (parsed + cached). Got: %v", len(m.contentHashes), m.contentHashes)
	}
	if got, want := m.contentHashes[cached], "prior-hash-for-cached"; got != want {
		t.Errorf("cached path content hash = %q, want %q (carried forward)", got, want)
	}
	if _, ok := m.contentHashes[parsed]; !ok {
		t.Errorf("parsed path missing from contentHashes")
	}
	if got, want := m.structuralFPs[cached], "prior-fp-for-cached"; got != want {
		t.Errorf("cached path structural fp = %q, want %q (carried forward)", got, want)
	}
}

// TestBuildManifestData_DropsDirtyUnparsedPaths covers deletions.
// A path that's marked dirty and isn't in parseResult is most likely
// gone from disk (the watcher fired but ParsePhase didn't pick it up).
// Carrying it forward would let it haunt the next manifest as if it
// were still cache-valid.
func TestBuildManifestData_DropsDirtyUnparsedPaths(t *testing.T) {
	tmp := t.TempDir()
	deleted := filepath.Join(tmp, "Gone.kt") // intentionally never written
	survivor := writeManifestTestFile(t, tmp, "Survivor.kt", "package demo\nclass Survivor\n")

	args := ProjectArgs{
		Paths:       []string{tmp},
		KotlinPaths: []string{deleted, survivor},
	}
	host := ProjectHostState{
		FindingsBundleCacheRoot: tmp,
		PriorContentHashes: map[string]string{
			deleted:  "prior-hash-deleted",
			survivor: "prior-hash-survivor",
		},
		PriorStructuralFPs: map[string]string{
			deleted:  "prior-fp-deleted",
			survivor: "prior-fp-survivor",
		},
		SourceSetDirty: []string{deleted},
	}
	parseResult := ParseResult{}

	m := buildManifestData(args, host, parseResult, scanner.RunFingerprint{}, true)
	if _, ok := m.contentHashes[deleted]; ok {
		t.Errorf("deleted dirty path should be dropped, but contentHashes contains it: %v", m.contentHashes)
	}
	if _, ok := m.contentHashes[survivor]; !ok {
		t.Errorf("non-dirty cached path should be carried forward; contentHashes=%v", m.contentHashes)
	}
}

func writeManifestTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
