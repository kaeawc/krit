package deadcode

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestBuildPlanClassifiesRemovableAndBlocked(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "src/Dead.kt",
			Line:    3,
			Rule:    "DeadCode",
			Message: "Public function 'dead' appears to be unused. It is not referenced from any other file.",
			Fix: &scanner.Fix{
				ByteMode:    true,
				StartByte:   10,
				EndByte:     24,
				Replacement: "",
			},
		},
		{
			File:    "src/Dead.kt",
			Line:    3,
			Rule:    "ModuleDeadCode",
			Message: "Public function 'dead' in module :app is not used by any module (including itself).",
		},
		{
			File:    "src/Api.kt",
			Line:    8,
			Rule:    "ModuleDeadCode",
			Message: "Public class 'Api' in module :lib is used only within the module. Consider making it internal.",
		},
	}

	plan := BuildPlan(findings)
	if len(plan.Candidates) != 1 {
		t.Fatalf("expected 1 removable candidate, got %d", len(plan.Candidates))
	}
	if len(plan.Blocked) != 1 {
		t.Fatalf("expected 1 blocked candidate, got %d", len(plan.Blocked))
	}
	if plan.Candidates[0].Kind != "function" || plan.Candidates[0].Name != "dead" {
		t.Fatalf("unexpected candidate: %+v", plan.Candidates[0])
	}
	if plan.Blocked[0].Reason != "visibility narrowing is not deletion" {
		t.Fatalf("unexpected blocked reason: %+v", plan.Blocked[0])
	}

	summary := plan.Summary()
	if summary.Declarations != 1 || summary.Files != 1 || summary.Blocked != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Kinds) != 1 || summary.Kinds[0].Kind != "function" || summary.Kinds[0].Count != 1 {
		t.Fatalf("unexpected kind breakdown: %+v", summary.Kinds)
	}
}

func TestBuildPlanColumnsMatchesSlicePlan(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "src/Dead.kt",
			Line:    3,
			Rule:    "DeadCode",
			Message: "Public function 'dead' appears to be unused. It is not referenced from any other file.",
			Fix: &scanner.Fix{
				ByteMode:    true,
				StartByte:   10,
				EndByte:     24,
				Replacement: "",
			},
		},
		{
			File:    "src/Dead.kt",
			Line:    3,
			Rule:    "ModuleDeadCode",
			Message: "Public function 'dead' in module :app is not used by any module (including itself).",
		},
		{
			File:    "src/Api.kt",
			Line:    8,
			Rule:    "ModuleDeadCode",
			Message: "Public class 'Api' in module :lib is used only within the module. Consider making it internal.",
		},
		{
			File:    "src/Blocked.kt",
			Line:    5,
			Rule:    "DeadCode",
			Message: "Private property 'unused' appears to be unused. It is not referenced from any other file.",
		},
	}

	columns := scanner.CollectFindings(findings)

	want := BuildPlan(findings)
	got := BuildPlanColumns(&columns)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan mismatch:\nwant: %+v\ngot:  %+v", want, got)
	}
}

func TestApplyUsesSharedFixEngine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dead.kt")
	content := "fun dead() = 1\n\nfun live() = 2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	removeEnd := strings.Index(content, "fun live()")
	if removeEnd < 0 {
		t.Fatal("failed to locate live function")
	}

	plan := BuildPlan([]scanner.Finding{
		{
			File:    path,
			Line:    1,
			Rule:    "DeadCode",
			Message: "Public function 'dead' appears to be unused. It is not referenced from any other file.",
			Fix: &scanner.Fix{
				ByteMode:    true,
				StartByte:   0,
				EndByte:     removeEnd,
				Replacement: "",
			},
		},
	})

	result := plan.Apply("")
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected apply errors: %v", result.Errors)
	}
	if result.Declarations != 1 || result.Files != 1 {
		t.Fatalf("unexpected apply result: %+v", result)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	if strings.Contains(got, "dead") {
		t.Fatalf("expected dead function to be removed, got: %s", got)
	}
	if !strings.Contains(got, "live") {
		t.Fatalf("expected live function to remain, got: %s", got)
	}
}

func TestBuildPlanColumnsApplyKeepsColumnarFixState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dead.kt")
	content := "fun dead() = 1\n\nfun live() = 2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	removeEnd := strings.Index(content, "fun live()")
	if removeEnd < 0 {
		t.Fatal("failed to locate live function")
	}

	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:    path,
			Line:    1,
			Rule:    "DeadCode",
			Message: "Public function 'dead' appears to be unused. It is not referenced from any other file.",
			Fix: &scanner.Fix{
				ByteMode:    true,
				StartByte:   0,
				EndByte:     removeEnd,
				Replacement: "",
			},
		},
	})

	plan := BuildPlanColumns(&columns)
	if len(plan.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(plan.Candidates))
	}

	result := plan.Apply("")
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected apply errors: %v", result.Errors)
	}
	if result.Declarations != 1 || result.Files != 1 {
		t.Fatalf("unexpected apply result: %+v", result)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	if strings.Contains(got, "dead") {
		t.Fatalf("expected dead function to be removed, got: %s", got)
	}
	if !strings.Contains(got, "live") {
		t.Fatalf("expected live function to remain, got: %s", got)
	}
}
