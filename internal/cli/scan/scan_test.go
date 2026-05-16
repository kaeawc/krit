package scan

import "testing"

// TestFilterGeneratedPathStringsDoesNotAliasInput pins the regression where
// filterGeneratedPathStrings used `paths[:0]`, mutating the caller's backing
// array. In runner_state.go the CLI runner aliases r.javaPathsForDispatch =
// r.allJavaPaths before filtering, so the [:0] rewrite left duplicate entries
// in r.allJavaPaths' tail. Those duplicates then flowed into the pipeline,
// causing cold runs to dispatch the same files multiple times and over-report
// findings against warm runs that loaded a deduped per-file cache.
func TestFilterGeneratedPathStringsDoesNotAliasInput(t *testing.T) {
	input := []string{
		"src/main/Foo.java",
		"build/generated/source/kapt/main/Generated1.java",
		"src/main/Bar.java",
		"build/generated/source/kapt/main/Generated2.java",
		"src/main/Baz.java",
	}
	original := append([]string(nil), input...)

	// Mimic the CLI's aliasing pattern.
	alias := input

	filtered := filterGeneratedPathStrings(alias)

	if got, want := len(filtered), 3; got != want {
		t.Fatalf("filtered len = %d, want %d", got, want)
	}
	for i, v := range input {
		if v != original[i] {
			t.Fatalf("filterGeneratedPathStrings mutated caller's slice at %d: got %q, want %q", i, v, original[i])
		}
	}
	seen := map[string]int{}
	for _, p := range input {
		seen[p]++
	}
	for p, c := range seen {
		if c > 1 {
			t.Fatalf("caller slice has duplicate %q after filter (count=%d) — filter must not share backing", p, c)
		}
	}
}
