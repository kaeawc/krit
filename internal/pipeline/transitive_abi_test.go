package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/scanner"
)

// buildTwoFileIndex returns a CodeIndex over two synthetic Kotlin
// files: declarer.kt declares class Widget (with method ring), and
// consumer.kt references Widget by name. The lookup map will report
// consumer.kt as a transitive dependent when the query name is
// "Widget" or "ring".
func buildTwoFileIndex(t *testing.T) (*scanner.CodeIndex, *scanner.File, *scanner.File) {
	t.Helper()
	dir := t.TempDir()
	declarer := mustParseKotlin(t, dir, "decl.kt", `package demo

class Widget {
    fun ring() {}
}
`)
	consumer := mustParseKotlin(t, dir, "cons.kt", `package demo

class Caller {
    fun use(w: Widget) { w.ring() }
}
`)
	idx := scanner.BuildIndex([]*scanner.File{declarer, consumer}, 1)
	if idx == nil {
		t.Fatal("BuildIndex returned nil")
	}
	return idx, declarer, consumer
}

func mustParseKotlin(t *testing.T, dir, name, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func TestMaybeExpandStaleByAbiDependents_NoStaleSeed(t *testing.T) {
	t.Parallel()
	// Empty stale slice short-circuits — nothing to expand.
	in := IndexInput{}
	got := maybeExpandStaleByAbiDependents(in, nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMaybeExpandStaleByAbiDependents_NoCodeIndex(t *testing.T) {
	t.Parallel()
	// Stale set present but no index — return stale unchanged so the
	// freshness gate falls back to per-file invalidation.
	in := IndexInput{StaleOraclePaths: []string{"/a.kt"}, PriorAbiHashes: map[string]string{"/a.kt": "x"}}
	got := maybeExpandStaleByAbiDependents(in, nil)
	if !reflect.DeepEqual(got, in.StaleOraclePaths) {
		t.Errorf("expected pass-through, got %v", got)
	}
}

func TestMaybeExpandStaleByAbiDependents_NoPriorAbi(t *testing.T) {
	t.Parallel()
	idx, _, _ := buildTwoFileIndex(t)
	in := IndexInput{StaleOraclePaths: []string{"/decl.kt"}}
	got := maybeExpandStaleByAbiDependents(in, idx)
	if !reflect.DeepEqual(got, in.StaleOraclePaths) {
		t.Errorf("expected pass-through, got %v", got)
	}
}

func TestExpandStaleByAbiDependents_AbiUnchanged_NoTransitive(t *testing.T) {
	t.Parallel()
	idx, declarer, _ := buildTwoFileIndex(t)
	currentABI := arch.HashAbiSignatures(arch.ExtractAbiSignatures([]*scanner.File{declarer}))
	// Prior ABI hash matches current → no ABI change → no transitive
	// expansion. consumer.kt must NOT be added.
	in := IndexInput{
		ParseResult:      ParseResult{KotlinFiles: []*scanner.File{declarer}},
		StaleOraclePaths: []string{declarer.Path},
		PriorAbiHashes:   map[string]string{declarer.Path: currentABI},
	}
	got := expandStaleByAbiDependents(in, idx)
	if !reflect.DeepEqual(got, in.StaleOraclePaths) {
		t.Errorf("ABI-unchanged stale path expanded: got %v, want %v", got, in.StaleOraclePaths)
	}
}

func TestExpandStaleByAbiDependents_AbiChanged_AddsTransitiveDependents(t *testing.T) {
	t.Parallel()
	idx, declarer, consumer := buildTwoFileIndex(t)
	// Prior ABI hash deliberately differs from current → simulates an
	// ABI change between runs → consumer.kt must be added because its
	// text references Widget/ring.
	in := IndexInput{
		ParseResult:      ParseResult{KotlinFiles: []*scanner.File{declarer, consumer}},
		StaleOraclePaths: []string{declarer.Path},
		PriorAbiHashes:   map[string]string{declarer.Path: "stale-hash-from-prior-run"},
	}
	got := expandStaleByAbiDependents(in, idx)
	sort.Strings(got)
	want := []string{consumer.Path, declarer.Path}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ABI-changed expansion = %v, want %v", got, want)
	}
}

func TestExpandStaleByAbiDependents_NewFile_NoPriorTreatedAsAbiChange(t *testing.T) {
	t.Parallel()
	// Brand-new file with no prior ABI hash should be treated as a
	// change for transitive purposes (its public surface is novel —
	// dependents may now reference it).
	idx, declarer, consumer := buildTwoFileIndex(t)
	in := IndexInput{
		ParseResult:      ParseResult{KotlinFiles: []*scanner.File{declarer, consumer}},
		StaleOraclePaths: []string{declarer.Path},
		PriorAbiHashes:   map[string]string{}, // declarer.kt absent
	}
	got := expandStaleByAbiDependents(in, idx)
	if len(got) < 2 {
		t.Errorf("new-file expansion missing transitive: %v", got)
	}
}

func TestExpandStaleByAbiDependents_UnparsedStalePath_PerFileScope(t *testing.T) {
	t.Parallel()
	// Stale path with no corresponding parsed file (deleted, off-scan)
	// can't compute current ABI — must stay per-file scope, not crash.
	idx, _, _ := buildTwoFileIndex(t)
	in := IndexInput{
		ParseResult:      ParseResult{KotlinFiles: nil}, // empty
		StaleOraclePaths: []string{"/ghost.kt"},
		PriorAbiHashes:   map[string]string{"/ghost.kt": "x"},
	}
	got := expandStaleByAbiDependents(in, idx)
	if !reflect.DeepEqual(got, in.StaleOraclePaths) {
		t.Errorf("ghost path expanded unexpectedly: %v", got)
	}
}
