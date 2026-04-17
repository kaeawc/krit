package rename

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestParseTarget(t *testing.T) {
	target, err := ParseTarget("com.example.OldName", "com.example.NewName")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if target.FromName != "OldName" {
		t.Fatalf("FromName = %q, want OldName", target.FromName)
	}
	if target.ToName != "NewName" {
		t.Fatalf("ToName = %q, want NewName", target.ToName)
	}
}

func TestBuildPlan(t *testing.T) {
	target, err := ParseTarget("com.example.OldName", "com.example.NewName")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}

	idx := scanner.BuildIndexFromData(
		[]scanner.Symbol{
			{Name: "OldName", Kind: "class", File: "src/main/kotlin/com/example/OldName.kt", Line: 3},
			{Name: "OtherName", Kind: "class", File: "src/main/kotlin/com/example/OtherName.kt", Line: 5},
		},
		[]scanner.Reference{
			{Name: "OldName", File: "src/main/kotlin/com/example/OldName.kt", Line: 3},
			{Name: "OldName", File: "src/main/kotlin/com/example/Feature.kt", Line: 10},
			{Name: "OldName", File: "src/main/java/com/example/FeatureJava.java", Line: 7},
			{Name: "OtherName", File: "src/main/kotlin/com/example/Feature.kt", Line: 14},
		},
	)

	plan := BuildPlan(idx, target)
	summary := plan.Summary()

	if summary.Declarations != 1 {
		t.Fatalf("Declarations = %d, want 1", summary.Declarations)
	}
	if summary.References != 3 {
		t.Fatalf("References = %d, want 3", summary.References)
	}
	if summary.Files != 3 {
		t.Fatalf("Files = %d, want 3", summary.Files)
	}
	if plan.CandidateCount() != 4 {
		t.Fatalf("CandidateCount = %d, want 4", plan.CandidateCount())
	}
	if got, want := plan.Files[0], "src/main/java/com/example/FeatureJava.java"; got != want {
		t.Fatalf("Files[0] = %q, want %q", got, want)
	}
}
