package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestPluginFindingsToColumns(t *testing.T) {
	cols := pluginFindingsToColumns([]oracle.PluginFinding{{
		File:       "Example.kt",
		Line:       3,
		Column:     7,
		RuleSet:    "myteam",
		RuleID:     "MyTeam/AvoidThing",
		Severity:   "warning",
		Message:    "avoid Thing",
		Confidence: 0.8,
		Fix: &oracle.PluginFix{
			StartLine:   3,
			EndLine:     3,
			Replacement: "replacement",
			Safety:      "cosmetic",
		},
	}})

	if cols.Len() != 1 {
		t.Fatalf("Len = %d, want 1", cols.Len())
	}
	if got := cols.RuleAt(0); got != "MyTeam/AvoidThing" {
		t.Errorf("RuleAt(0) = %q", got)
	}
	if got := cols.RuleSetAt(0); got != "myteam" {
		t.Errorf("RuleSetAt(0) = %q", got)
	}
	if got := cols.MessageAt(0); got != "avoid Thing" {
		t.Errorf("MessageAt(0) = %q", got)
	}
	fix := cols.FixAt(0)
	if fix == nil || fix.Replacement != "replacement" {
		t.Fatalf("FixAt(0) = %#v", fix)
	}
}

func TestFormatPluginErrorsDeterministic(t *testing.T) {
	got := formatPluginErrors(map[string]string{
		"ZRule": "z failed",
		"ARule": "a failed",
	})
	want := "ARule: a failed; ZRule: z failed"
	if got != want {
		t.Fatalf("formatPluginErrors = %q, want %q", got, want)
	}
}

func TestPluginFindingsRespectSourceSuppressions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte(`@Suppress("MyTeam/AvoidThing")
fun example() {
    legacy()
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	parse, err := ParsePhase{}.Run(context.Background(), ParseInput{
		Paths:       []string{path},
		ActiveRules: []*api.Rule{api.FakeRule("AnyRule")},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parse.KotlinFiles) != 1 {
		t.Fatalf("KotlinFiles = %d, want 1", len(parse.KotlinFiles))
	}
	cols := pluginFindingsToColumns([]oracle.PluginFinding{{
		File:       path,
		Line:       3,
		Column:     5,
		RuleSet:    "myteam",
		RuleID:     "MyTeam/AvoidThing",
		Severity:   "warning",
		Message:    "avoid Thing",
		Confidence: 0.8,
	}})

	filtered := applySuppressionColumns(&cols, parse.KotlinFiles)
	if filtered.Len() != 0 {
		t.Fatalf("suppressed plugin finding was kept: %+v", filtered.Findings())
	}

	keptCols := pluginFindingsToColumns([]oracle.PluginFinding{{
		File:       path,
		Line:       3,
		Column:     5,
		RuleSet:    "myteam",
		RuleID:     "MyTeam/OtherThing",
		Severity:   "warning",
		Message:    "other Thing",
		Confidence: 0.8,
	}})
	filtered = applySuppressionColumns(&keptCols, parse.KotlinFiles)
	if filtered.Len() != 1 {
		t.Fatalf("unsuppressed plugin finding Len = %d, want 1", filtered.Len())
	}
}
