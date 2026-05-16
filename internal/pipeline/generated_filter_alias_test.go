package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestFilterGeneratedSourcePathsDoesNotAliasInput pins the regression that
// caused cold/warm finding-count divergence on the JetBrains/kotlin corpus:
// filterGeneratedSourcePaths used `paths[:0]`, so it mutated the caller's
// backing array in place. callers in runProjectAnalysis pass args.KotlinPaths
// and args.JavaPaths (owned by the CLI runner's r.files / r.allJavaPaths);
// the in-place rewrite left dupes in the runner's slices, and downstream
// parse + dispatch then processed the same files multiple times on cold runs
// (warm runs skipped dispatch via the per-file cache and so reported fewer
// findings).
func TestFilterGeneratedSourcePathsDoesNotAliasInput(t *testing.T) {
	input := []string{
		"src/Foo.kt",
		"build/generated/source/kapt/Generated1.kt",
		"src/Bar.kt",
		"build/generated/source/kapt/Generated2.kt",
		"src/Baz.kt",
	}
	original := append([]string(nil), input...)
	alias := input

	filtered := filterGeneratedSourcePaths(alias, false)

	if got, want := len(filtered), 3; got != want {
		t.Fatalf("filtered len = %d, want %d", got, want)
	}
	for i, v := range input {
		if v != original[i] {
			t.Fatalf("filterGeneratedSourcePaths mutated caller's slice at %d: got %q, want %q", i, v, original[i])
		}
	}
}

// TestFilterGeneratedSourceFilesWithAllowlistDoesNotAliasInput pins the
// matching [:0] hazard on the *scanner.File variant.
func TestFilterGeneratedSourceFilesWithAllowlistDoesNotAliasInput(t *testing.T) {
	input := []*scanner.File{
		{Path: "src/Foo.kt"},
		{Path: "build/generated/source/kapt/Generated1.kt"},
		{Path: "src/Bar.kt"},
		{Path: "build/generated/source/kapt/Generated2.kt"},
		{Path: "src/Baz.kt"},
	}
	originalPaths := make([]string, len(input))
	for i, f := range input {
		originalPaths[i] = f.Path
	}
	alias := input

	filtered, dropped := filterGeneratedSourceFilesWithAllowlist(alias, nil)

	if got, want := len(filtered), 3; got != want {
		t.Fatalf("filtered len = %d, want %d", got, want)
	}
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2", dropped)
	}
	for i, f := range input {
		if f.Path != originalPaths[i] {
			t.Fatalf("filterGeneratedSourceFilesWithAllowlist mutated caller's slice at %d: got %q, want %q", i, f.Path, originalPaths[i])
		}
	}
}
