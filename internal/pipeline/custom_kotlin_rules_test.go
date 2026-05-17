package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

func TestSelectPluginRulesSkipsDisabledAndCollectsOptions(t *testing.T) {
	loaded := []oracle.PluginRuleDescriptor{
		{RuleID: "acme.NoTodo"},
		{RuleID: "acme.NoFixme"},
		{RuleID: "acme.OptInOnly"},
		{RuleID: ""}, // dropped: empty IDs come from malformed jars
	}
	cfg := config.NewConfigFromData(map[string]interface{}{
		"pluginRules": map[string]interface{}{
			"acme.NoTodo": map[string]interface{}{
				"active": false,
			},
			"acme.NoFixme": map[string]interface{}{
				"options": map[string]interface{}{
					"maxLineLength": 100,
				},
			},
			"acme.OptInOnly": map[string]interface{}{
				"active": true,
			},
		},
	})

	ids, opts := selectPluginRules(loaded, cfg)

	wantIDs := []string{"acme.NoFixme", "acme.OptInOnly"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("ruleIDs = %v, want %v", ids, wantIDs)
	}
	wantOpts := map[string]map[string]interface{}{
		"acme.NoFixme": {"maxLineLength": 100},
	}
	if !reflect.DeepEqual(opts, wantOpts) {
		t.Fatalf("ruleOptions = %v, want %v", opts, wantOpts)
	}
}

func TestSelectPluginRulesNilConfigKeepsEveryRuleWithoutOptions(t *testing.T) {
	loaded := []oracle.PluginRuleDescriptor{
		{RuleID: "acme.A"},
		{RuleID: "acme.B"},
	}
	ids, opts := selectPluginRules(loaded, nil)
	if !reflect.DeepEqual(ids, []string{"acme.A", "acme.B"}) {
		t.Fatalf("ruleIDs = %v, want passthrough", ids)
	}
	if len(opts) != 0 {
		t.Fatalf("ruleOptions = %v, want empty", opts)
	}
}

func TestReportPluginDiagnosticsWarnsAndAggregatesErrors(t *testing.T) {
	warn := &bytes.Buffer{}
	reporter := &diag.Reporter{Warning: warn}

	err := reportPluginDiagnostics(reporter, []oracle.PluginLoadDiagnostic{
		{
			Jar: "/tmp/z.jar", Level: oracle.PluginDiagError,
			RuleSDKVersion: "1.0.0", DaemonSDKVersion: "2.0.0",
			Message: "rule jar built against krit-rule-api 1.0.0 is incompatible with daemon krit-rule-api 2.0.0 (major version mismatch); rebuild against 2.0.0",
		},
		{
			Jar: "/tmp/a.jar", Level: oracle.PluginDiagWarn,
			RuleSDKVersion: "1.2.0", DaemonSDKVersion: "1.3.0",
			Message: "minor version differs",
		},
		{
			Jar: "/tmp/m.jar", Level: oracle.PluginDiagError,
			RuleSDKVersion: "", DaemonSDKVersion: "1.3.0",
			Message: "missing manifest",
		},
	})
	if err == nil {
		t.Fatal("expected error from PluginDiagError diagnostics")
	}
	msg := err.Error()
	// Errors are sorted by full formatted line so the output is
	// deterministic across map iteration.
	wantOrder := []string{"/tmp/m.jar", "/tmp/z.jar"}
	for i, jar := range wantOrder {
		idx := strings.Index(msg, jar)
		if idx < 0 {
			t.Fatalf("error missing %s: %q", jar, msg)
		}
		if i > 0 && idx < strings.Index(msg, wantOrder[i-1]) {
			t.Fatalf("errors not sorted: %q", msg)
		}
	}
	if !strings.Contains(msg, "custom rule jar(s) failed to load") {
		t.Errorf("error missing remediation prefix: %q", msg)
	}
	if !strings.Contains(msg, "rebuild against the daemon's krit-rule-api version") {
		t.Errorf("error missing SDK-rebuild hint: %q", msg)
	}
	if !strings.Contains(msg, "remove unsupported capability declarations") {
		t.Errorf("error missing capability remediation hint: %q", msg)
	}

	gotWarn := warn.String()
	if !strings.Contains(gotWarn, "warn: krit-rule-api: /tmp/a.jar: minor version differs") {
		t.Errorf("warn output missing formatted warn line: %q", gotWarn)
	}
	if strings.Contains(gotWarn, "/tmp/z.jar") || strings.Contains(gotWarn, "/tmp/m.jar") {
		t.Errorf("errors leaked into warn stream: %q", gotWarn)
	}
}

func TestReportPluginDiagnosticsNoOpOnEmpty(t *testing.T) {
	warn := &bytes.Buffer{}
	reporter := &diag.Reporter{Warning: warn}
	if err := reportPluginDiagnostics(reporter, nil); err != nil {
		t.Fatalf("nil diagnostics should be a no-op: %v", err)
	}
	if warn.Len() != 0 {
		t.Errorf("nil diagnostics wrote to reporter: %q", warn.String())
	}
}

func TestReportPluginDiagnosticsUnknownLevelStillSurfaces(t *testing.T) {
	warn := &bytes.Buffer{}
	reporter := &diag.Reporter{Warning: warn}
	err := reportPluginDiagnostics(reporter, []oracle.PluginLoadDiagnostic{
		{Jar: "/tmp/x.jar", Level: oracle.PluginDiagLevel("info"), Message: "fyi"},
	})
	if err != nil {
		t.Fatalf("unknown-level diagnostic should not be fatal: %v", err)
	}
	if !strings.Contains(warn.String(), "/tmp/x.jar") {
		t.Errorf("unknown-level diagnostic should still print via warn stream: %q", warn.String())
	}
}

func TestPluginLoadDiagnosticFormat(t *testing.T) {
	d := oracle.PluginLoadDiagnostic{
		Jar:     "/tmp/acme.jar",
		Level:   oracle.PluginDiagWarn,
		Message: "minor version differs",
	}
	got := d.Format()
	want := "warn: krit-rule-api: /tmp/acme.jar: minor version differs"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
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

func TestPluginFindingsRoundTripThroughBaseline(t *testing.T) {
	// Issue #307: plugin findings must produce stable BaselineIDs that
	// survive a --create-baseline / --baseline round-trip the same way
	// built-in findings do. Guards against a future regression that
	// drops plugin findings before the baseline writer without needing
	// the JVM daemon.
	dir := t.TempDir()
	cols := pluginFindingsToColumns([]oracle.PluginFinding{{
		File:    filepath.Join(dir, "Example.kt"),
		Line:    1,
		Column:  1,
		RuleSet: "myteam",
		RuleID:  "MyTeam/AvoidThing",
		Message: "avoid thing",
	}})
	if cols.Len() != 1 {
		t.Fatalf("pluginFindingsToColumns Len = %d, want 1", cols.Len())
	}

	baselinePath := filepath.Join(dir, "baseline.xml")
	if err := scanner.WriteBaselineColumns(baselinePath, &cols, dir); err != nil {
		t.Fatalf("WriteBaselineColumns: %v", err)
	}

	baseline, err := scanner.LoadBaseline(baselinePath)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if len(baseline.CurrentIssues) != 1 {
		t.Fatalf("baseline CurrentIssues = %d, want 1: %+v",
			len(baseline.CurrentIssues), baseline.CurrentIssues)
	}
	for id := range baseline.CurrentIssues {
		if !strings.HasPrefix(id, "MyTeam/AvoidThing:") {
			t.Errorf("baseline ID %q does not start with plugin rule prefix", id)
		}
	}

	filtered := scanner.FilterColumnsByBaseline(&cols, baseline, dir)
	if filtered.Len() != 0 {
		t.Fatalf("baseline did not suppress plugin finding: Len = %d, findings = %+v",
			filtered.Len(), filtered.Findings())
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
