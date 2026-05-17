package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestRunKotlinPluginRulesRequiresDaemonWhenJarsConfigured(t *testing.T) {
	args := ProjectArgs{CustomRuleJars: []string{"/tmp/does-not-exist.jar"}}
	indexResult := IndexResult{}
	crossFile := &CrossFileResult{}

	err := runKotlinPluginRulesAndMerge(context.Background(), args, ProjectHostState{}, indexResult, crossFile, false)
	if err == nil {
		t.Fatal("expected error when CustomRuleJars set without daemon, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "krit-types daemon") {
		t.Errorf("error should name the failing daemon: %q", msg)
	}
	if !strings.Contains(msg, "--daemon") {
		t.Errorf("error should suggest --daemon flag: %q", msg)
	}
	if !strings.Contains(msg, "KRIT_TYPES_JAR") {
		t.Errorf("error should point at KRIT_TYPES_JAR recovery path: %q", msg)
	}
}

func TestRunKotlinPluginRulesNoOpWhenNoJars(t *testing.T) {
	if err := runKotlinPluginRulesAndMerge(context.Background(), ProjectArgs{}, ProjectHostState{}, IndexResult{}, &CrossFileResult{}, false); err != nil {
		t.Fatalf("unexpected error with empty jars: %v", err)
	}
}

func TestRunKotlinPluginRulesNoOpWhenBundleHit(t *testing.T) {
	args := ProjectArgs{CustomRuleJars: []string{"/tmp/does-not-exist.jar"}}
	if err := runKotlinPluginRulesAndMerge(context.Background(), args, ProjectHostState{}, IndexResult{}, &CrossFileResult{}, true); err != nil {
		t.Fatalf("unexpected error on bundle hit: %v", err)
	}
}

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

func TestPluginFixToScannerPopulatesSafety(t *testing.T) {
	tests := []struct {
		safety string
		want   uint8
	}{
		{"cosmetic", uint8(rules.FixCosmetic)},
		{"idiomatic", uint8(rules.FixIdiomatic)},
		{"semantic", uint8(rules.FixSemantic)},
		{"", uint8(rules.FixSemantic)},        // unset → conservative
		{"unknown", uint8(rules.FixSemantic)}, // unrecognized → conservative
	}
	for _, tt := range tests {
		t.Run(tt.safety, func(t *testing.T) {
			got := pluginFixToScanner(&oracle.PluginFix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "x",
				Safety:      tt.safety,
			})
			if got == nil {
				t.Fatal("pluginFixToScanner = nil")
			}
			if got.Safety != tt.want {
				t.Errorf("Safety = %d, want %d", got.Safety, tt.want)
			}
		})
	}

	if got := pluginFixToScanner(nil); got != nil {
		t.Errorf("pluginFixToScanner(nil) = %#v, want nil", got)
	}
}

func TestFixupGatesPluginSemanticFixUnderCosmeticLevel(t *testing.T) {
	// Plugin rule reports a SEMANTIC fix; --fix-level cosmetic must
	// strip it just like any built-in FixSemantic. We seed scanner.Fix
	// the same way pluginFixToScanner does so the registry doesn't
	// know the rule but the fix carries Safety directly.
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	original := "val foo = 1\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cols := pluginFindingsToColumns([]oracle.PluginFinding{{
		File:       path,
		Line:       1,
		Column:     1,
		RuleSet:    "myteam",
		RuleID:     "MyTeam/SemanticRule",
		Severity:   "warning",
		Message:    "semantic plugin fix",
		Confidence: 0.9,
		Fix: &oracle.PluginFix{
			StartLine:   1,
			EndLine:     1,
			Replacement: "val bar = 1",
			Safety:      "semantic",
		},
	}})

	out, err := (FixupPhase{}).Run(context.Background(), FixupInput{
		CrossFileResult: CrossFileResult{
			DispatchResult: DispatchResult{Findings: cols},
		},
		Apply:       true,
		MaxFixLevel: rules.FixCosmetic,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.AppliedFixes != 0 {
		t.Errorf("AppliedFixes = %d, want 0 (semantic plugin fix must be stripped)", out.AppliedFixes)
	}
	if out.StrippedByLevel != 1 {
		t.Errorf("StrippedByLevel = %d, want 1", out.StrippedByLevel)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != original {
		t.Errorf("file changed: got %q, want %q", string(got), original)
	}
}

func TestFixupAppliesPluginCosmeticFixUnderCosmeticLevel(t *testing.T) {
	// Counterpart to the semantic-strip test: a plugin fix declared
	// COSMETIC must survive the same --fix-level cosmetic gate even
	// though the rule isn't in the built-in registry (where the
	// fallback would otherwise classify it as FixSemantic).
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(path, []byte("val foo = 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cols := pluginFindingsToColumns([]oracle.PluginFinding{{
		File:     path,
		Line:     1,
		Column:   1,
		RuleSet:  "myteam",
		RuleID:   "MyTeam/CosmeticRule",
		Severity: "warning",
		Message:  "cosmetic plugin fix",
		Fix: &oracle.PluginFix{
			StartLine:   1,
			EndLine:     1,
			Replacement: "val bar = 1",
			Safety:      "cosmetic",
		},
	}})

	out, err := (FixupPhase{}).Run(context.Background(), FixupInput{
		CrossFileResult: CrossFileResult{
			DispatchResult: DispatchResult{Findings: cols},
		},
		Apply:       true,
		MaxFixLevel: rules.FixCosmetic,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.AppliedFixes != 1 {
		t.Errorf("AppliedFixes = %d, want 1 (cosmetic plugin fix must apply)", out.AppliedFixes)
	}
	if out.StrippedByLevel != 0 {
		t.Errorf("StrippedByLevel = %d, want 0", out.StrippedByLevel)
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
