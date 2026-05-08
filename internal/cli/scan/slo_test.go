package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestEvaluateSLOs_EmitsWhenWarningDensityExceedsThreshold(t *testing.T) {
	graph, files := testSLOGraph(t, 1000)
	limit := 5.0
	findings := make([]scanner.Finding, 0, 6)
	for i := 0; i < 6; i++ {
		findings = append(findings, scanner.Finding{
			File:     files[":core"][0].Path,
			Rule:     "SomeRule",
			Severity: "warning",
		})
	}

	got := evaluateSLOs([]config.SLOConfig{{
		Module:             ":core",
		MaxWarningsPerKLOC: &limit,
	}}, graph, files, findings)

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Rule != sloRuleName || got[0].RuleSet != sloRuleSet || got[0].Severity != "warning" {
		t.Fatalf("unexpected SLO finding metadata: %#v", got[0])
	}
	if !strings.Contains(got[0].Message, `Module ":core" has 6.0 warnings per 1k LOC, above SLO 5.0.`) {
		t.Fatalf("unexpected message: %q", got[0].Message)
	}
}

func TestEvaluateSLOs_NoFindingWhenWithinBudget(t *testing.T) {
	graph, files := testSLOGraph(t, 1000)
	limit := 5.0
	findings := []scanner.Finding{{
		File:     files[":core"][0].Path,
		Rule:     "SomeRule",
		Severity: "warning",
	}}

	got := evaluateSLOs([]config.SLOConfig{{
		Module:             ":core",
		MaxWarningsPerKLOC: &limit,
	}}, graph, files, findings)

	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestEvaluateSLOs_ExcludesTestSourcesFromLOC(t *testing.T) {
	graph, files := testSLOGraph(t, 1000)
	testPath := filepath.Join(graph.Modules[":core"].Dir, "src", "test", "kotlin", "CoreTest.kt")
	files[":core"] = append(files[":core"], &scanner.File{Path: testPath, Lines: make([]string, 1000)})
	limit := 5.0
	findings := make([]scanner.Finding, 0, 6)
	for i := 0; i < 6; i++ {
		findings = append(findings, scanner.Finding{
			File:     files[":core"][0].Path,
			Rule:     "SomeRule",
			Severity: "warning",
		})
	}

	got := evaluateSLOs([]config.SLOConfig{{
		Module:             ":core",
		MaxWarningsPerKLOC: &limit,
	}}, graph, files, findings)

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !strings.Contains(got[0].Message, "6.0 warnings per 1k LOC") {
		t.Fatalf("test LOC appears to have been counted: %q", got[0].Message)
	}
}

func TestEvaluateSLOs_EmitsErrorsIndependently(t *testing.T) {
	graph, files := testSLOGraph(t, 1000)
	limit := 0.0
	findings := []scanner.Finding{{
		File:     files[":core"][0].Path,
		Rule:     "SomeRule",
		Severity: "error",
	}}

	got := evaluateSLOs([]config.SLOConfig{{
		Module:           ":core",
		MaxErrorsPerKLOC: &limit,
	}}, graph, files, findings)

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !strings.Contains(got[0].Message, "1.0 errors per 1k LOC") {
		t.Fatalf("unexpected message: %q", got[0].Message)
	}
}

func testSLOGraph(t *testing.T, loc int) (*module.Graph, map[string][]*scanner.File) {
	t.Helper()
	root := t.TempDir()
	coreDir := filepath.Join(root, "core")
	graph := module.NewModuleGraph(root)
	graph.Modules[":core"] = &module.Module{Path: ":core", Dir: coreDir}
	file := &scanner.File{
		Path:  filepath.Join(coreDir, "src", "main", "kotlin", "Core.kt"),
		Lines: make([]string, loc),
	}
	return graph, map[string][]*scanner.File{":core": {file}}
}
