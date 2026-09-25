package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

func TestShouldFallbackToAnalyzeAll(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		pruned bool
		want   bool
	}{
		{name: "clean merge", want: false},
		{name: "pruned cached files", pruned: true, want: true},
		{name: "merge error", err: errors.New("merge failed"), want: true},
		{name: "error and pruning", err: errors.New("merge failed"), pruned: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldFallbackToAnalyzeAll(tt.err, tt.pruned); got != tt.want {
				t.Fatalf("shouldFallbackToAnalyzeAll(%v, %v) = %v, want %v", tt.err, tt.pruned, got, tt.want)
			}
		})
	}
}

// A pruned merge can still carry a deleted file's Dependencies entries, so
// the full fallback result must replace types.json; otherwise the next
// partial run unions the stale entries straight back in.
func TestAnalyzeAllAndPersist_StopsDeletedFileFactsReturningOnNextMerge(t *testing.T) {
	dir := t.TempDir()
	typesPath := filepath.Join(dir, "types.json")
	gone := filepath.Join(dir, "Gone.kt")
	other := filepath.Join(dir, "Other.kt")
	for _, p := range []string{gone, other} {
		if err := os.WriteFile(p, []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed := &oracle.Data{
		Files: map[string]*oracle.File{
			gone:  {Package: "p"},
			other: {Package: "p"},
		},
		Dependencies: map[string]*oracle.Class{"p.Gone": {FQN: "p.Gone"}},
	}
	if err := oracle.WriteTypesJSON(typesPath, seed); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	fresh := func() *oracle.Data {
		return &oracle.Data{Files: map[string]*oracle.File{other: {Package: "p"}}}
	}

	_, pruned, err := oracle.MergeFreshIntoCachedTypes(typesPath, fresh())
	if err != nil || !pruned {
		t.Fatalf("first merge: pruned=%v err=%v, want a prune", pruned, err)
	}
	full := &oracle.Data{Files: map[string]*oracle.File{other: {Package: "p"}}}
	od, err := analyzeAllAndPersist(func() (*oracle.Data, error) { return full, nil }, typesPath, t.Logf)
	if err != nil || od != full {
		t.Fatalf("analyzeAllAndPersist = %v, %v", od, err)
	}

	merged, pruned, err := oracle.MergeFreshIntoCachedTypes(typesPath, fresh())
	if err != nil {
		t.Fatal(err)
	}
	if pruned {
		t.Fatal("second merge pruned again; the fallback result was not persisted")
	}
	if _, ok := merged.Dependencies["p.Gone"]; ok {
		t.Fatal("deleted file's dependency came back on the next partial merge")
	}
	if _, ok := merged.Files[gone]; ok {
		t.Fatal("deleted file's facts came back on the next partial merge")
	}
}

func TestAnalyzeAllAndPersist_AnalyzeErrorLeavesTypesJSON(t *testing.T) {
	typesPath := filepath.Join(t.TempDir(), "types.json")
	want := errors.New("daemon down")
	if _, err := analyzeAllAndPersist(func() (*oracle.Data, error) { return nil, want }, typesPath, t.Logf); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if _, err := os.Stat(typesPath); !os.IsNotExist(err) {
		t.Fatalf("types.json written after a failed analysis: %v", err)
	}
}
