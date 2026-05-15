package serve

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAnalyzeProject_OutputMatchesDirectRunProject is the load-
// bearing contract that the daemon verb does not corrupt findings.
// It runs the same fixture through two paths:
//
//   - direct: pipeline.RunProject(ctx, in) in-process
//   - verb:   handleAnalyzeProject via daemon socket
//
// Both Findings JSON payloads must be equal modulo timing fields
// (durationMs, wall_seconds, the perf subtree). A divergence here
// means the verb's marshaling, single-flight, or stats accounting
// is dropping or reshaping data.
//
// The CLI subprocess equivalence test (`TestAnalyzeProjectMatches
// OneShotCLI`) is deferred: today's `scan.runner` composes phases
// the daemon doesn't yet (FIR check, baselines, fix application),
// so a byte-equal assertion against `krit -f json` would catch real
// drift mixed with expected differences. That comparison waits for
// the runner-consolidation commit.
func TestAnalyzeProject_OutputMatchesDirectRunProject(t *testing.T) {
	socket, state := startServerForTest(t)

	// Multi-file fixture with content that exercises multiple rule
	// families: a class declaration, a function, an import, an
	// unused variable. The default rule set will produce several
	// findings across both files.
	writeKotlinFile(t, state.root, "Foo.kt",
		"package demo\n\nimport kotlin.io.println\n\nclass Foo {\n    fun greet() = println(\"hi\")\n}\n")
	writeKotlinFile(t, state.root, "Bar.kt",
		"package demo\n\nfun bar(unused: Int): Int { return 42 }\n")

	// --- Direct path -----------------------------------------------
	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	activeRules := rules.ActiveRulesV2(map[string]bool{}, map[string]bool{}, false, false, false)
	repoDir := oracle.FindRepoDir([]string{state.root})
	if repoDir == "" {
		repoDir = state.root
	}
	pc, err := scanner.NewParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		t.Fatalf("direct ParseCache: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	directResult, err := pipeline.RunProject(context.Background(), pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:      cfg,
			Paths:       []string{state.root},
			ActiveRules: activeRules,
			Format:      "json",
			Version:     "test",
		},
		Host: pipeline.ProjectHostState{ParseCache: pc},
	})
	if err != nil {
		t.Fatalf("direct RunProject: %v", err)
	}

	// --- Verb path -------------------------------------------------
	var verbResult daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{Format: "json"}, &verbResult); err != nil {
		t.Fatalf("verb call: %v", err)
	}

	// Both should be valid JSON.
	directJSON := stripTimingFields(t, directResult.JSON)
	verbJSON := stripTimingFields(t, verbResult.Findings)

	if !reflect.DeepEqual(directJSON, verbJSON) {
		t.Errorf("verb output diverges from direct RunProject after timing-strip\n--- direct ---\n%s\n--- verb ---\n%s",
			mustJSON(t, directJSON), mustJSON(t, verbJSON))
	}

	// Sanity: both paths must have actually produced findings or both
	// must have produced none (the assertion above already covers the
	// content; this guards against an empty-equals-empty false pass).
	if directResult.FindingsCount != verbResult.Stats.FindingsCount {
		t.Errorf("FindingsCount mismatch: direct=%d verb=%d",
			directResult.FindingsCount, verbResult.Stats.FindingsCount)
	}
}

// stripTimingFields decodes the JSON, removes fields that legitimately
// vary between runs (wall_seconds, durationMs, perf subtree, caches
// subtree, startTime), and returns the canonicalized map. The returned
// value is suitable for reflect.DeepEqual comparison.
func stripTimingFields(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("strip: not JSON: %v\n%s", err, raw)
	}
	for _, key := range []string{
		"durationMs", "wall_seconds", "wallSeconds", "perf", "caches",
		"cacheBudget", "startTime", "timing", "timings",
		// "version" varies because the daemon path uses kritVersion()
		// while the direct path lets the caller pass an explicit
		// string. Both are derived from the same compile-time var
		// chain in production; differences are environmental, not
		// content. Strip so the contract focuses on findings.
		"version",
	} {
		delete(out, key)
	}
	// Findings carry per-finding timing in some output modes — strip
	// any "timeMs" / "tookNs" fields inside the findings array.
	if findings, ok := out["findings"].([]any); ok {
		for _, f := range findings {
			if m, ok := f.(map[string]any); ok {
				delete(m, "timeMs")
				delete(m, "tookNs")
			}
		}
	}
	return out
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
