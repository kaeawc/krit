package scan

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestComputeStaleOraclePaths_FiltersGeneratedWhenDisabled is the
// regression guard for the warm-rerun freshness-gate phantom-miss bug.
// Before the fix, the gate compared every .kt path against the bundle
// manifest — including files under /generated/ that the parse phase
// already filters out. Those paths weren't in the manifest (parse had
// dropped them before the manifest write), so the gate marked them as
// stale, the oracle layer treated them as ForcedMisses, and krit-fir
// launched a one-shot JVM to analyze ~98 files on every warm rerun
// against the kotlin repo. ~80s of pointless JVM startup cost per
// warm scan.
//
// The fix applies the same /generated/ filter to the gate's input
// that the parse phase will apply. With includeGenerated=false, paths
// under /generated/ never reach the manifest comparison.
//
// This test asserts the filter is applied symmetrically: we never
// load a manifest (no prior bundle) so the function short-circuits to
// nil for the non-generated cases — what we care about is that the
// generated paths are pruned from the input slice the gate considers.
// We exercise that via a strings.Contains check on the function's
// own filter helper, since the gate's full path requires a real
// repoDir + cache layout to load a manifest.
func TestFilterGeneratedPathStrings_DropsGeneratedKotlin(t *testing.T) {
	t.Helper()
	in := []string{
		"src/main/kotlin/Foo.kt",
		filepath.FromSlash("a/b/generated/_ArraysNative.kt"),
		filepath.FromSlash("kotlin-native/runtime/src/main/kotlin/generated/_CollectionsNative.kt"),
		"src/main/kotlin/Bar.kt",
	}
	out := filterGeneratedPathStrings(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 retained paths after /generated/ filter, got %d: %v", len(out), out)
	}
	for _, p := range out {
		if strings.Contains(filepath.ToSlash(p), "/generated/") {
			t.Errorf("filter retained generated path: %q", p)
		}
	}
}
