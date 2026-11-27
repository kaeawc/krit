package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// A source set that no longer matches the index rejects warm cross-findings
// reuse before any Java file is hashed (#1850 review); a matching set still
// compares contents.
func TestCoversWarmSources_ChecksPathsBeforeHashing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.java")
	b := filepath.Join(dir, "B.java")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("class X {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var meta scanner.CrossFileCacheMeta
	entries, err := json.Marshal(map[string]any{"entries": []map[string]string{{"Path": a, "Hash": hashSourceFile(a)}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(entries, &meta); err != nil {
		t.Fatal(err)
	}
	var hashed []string
	testHookHashSourceFiles = func(paths []string) { hashed = append(hashed, paths...) }
	t.Cleanup(func() { testHookHashSourceFiles = nil })

	if coversWarmSources(meta, []string{a, b}, warmAnalysisCachePlan{}) {
		t.Fatal("an index of A.java alone covered A.java and B.java")
	}
	if len(hashed) != 0 {
		t.Fatalf("a source-set mismatch hashed %v before rejecting reuse", hashed)
	}
	if !coversWarmSources(meta, []string{a}, warmAnalysisCachePlan{}) {
		t.Fatal("an index of unchanged A.java did not cover it")
	}
	if len(hashed) != 1 || hashed[0] != a {
		t.Fatalf("a matching source set hashed %v, want [%s]", hashed, a)
	}
	if err := os.WriteFile(a, []byte("class Y {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if coversWarmSources(meta, []string{a}, warmAnalysisCachePlan{}) {
		t.Fatal("an index of A.java covered it after an edit")
	}
}
