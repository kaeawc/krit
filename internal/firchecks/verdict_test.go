package firchecks

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestApplyVerdictConfirmOnly(t *testing.T) {
	for _, rule := range []string{
		"OptInMarkerExposedPublicly",
		"RunBlockingInTest",
		"TestWithoutAssertion",
		"UnspecifiedImmutableFlag",
	} {
		t.Run(rule, func(t *testing.T) {
			testApplyVerdictConfirmOnly(t, rule)
		})
	}
}

func testApplyVerdictConfirmOnly(t *testing.T, rule string) {
	t.Helper()
	const file = "/src/Test.kt"
	goFinding := scanner.Finding{File: file, Line: 3, StartByte: 10, EndByte: 50, Rule: rule, Message: "Go"}
	goWithoutRange := goFinding
	goWithoutRange.StartByte, goWithoutRange.EndByte = 0, 0
	for _, test := range []struct {
		name       string
		goFindings []scanner.Finding
		firLine    int
		want       []scanner.Finding
		wantStats  RuleVerdict
	}{
		{"FIR-only dropped", nil, 3, nil, RuleVerdict{ConfirmOnlyDropped: 1}},
		{"same-line Go finding confirmed", []scanner.Finding{goFinding}, 3, []scanner.Finding{goFinding}, RuleVerdict{Confirmed: 1}},
		{"same-line Go finding without range confirmed", []scanner.Finding{goWithoutRange}, 3, []scanner.Finding{goWithoutRange}, RuleVerdict{Confirmed: 1}},
		{"unconfirmed Go finding dropped", []scanner.Finding{goFinding}, 0, nil, RuleVerdict{GoDropped: 1}},
		{"overlap on different line dropped", []scanner.Finding{goFinding}, 4, nil, RuleVerdict{GoDropped: 1, ConfirmOnlyDropped: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := &Result{Rules: []string{rule}}
			if test.firLine != 0 {
				result.Findings = []scanner.Finding{{File: file, Line: test.firLine, Rule: rule, Message: "FIR"}}
			}
			got, stats := ApplyVerdict(VerdictInput{Go: test.goFindings, FIR: result, Requested: []string{file}})
			if !reflect.DeepEqual(describe(got), describe(test.want)) || *stats.Rules[rule] != test.wantStats {
				t.Fatalf("findings = %v, stats = %+v; want %v, %+v", got, *stats.Rules[rule], test.want, test.wantStats)
			}
		})
	}
}

func TestRunBlockingInTestConfirmOnly(t *testing.T) {
	const file = "/src/Test.kt"
	const rule = "RunBlockingInTest"
	goFinding := scanner.Finding{File: file, Line: 3, Rule: rule, Message: "Go"}
	for _, test := range []struct {
		name       string
		goFindings []scanner.Finding
		firLine    int
		want       []scanner.Finding
		wantStats  RuleVerdict
	}{
		{"FIR-only dropped", nil, 3, nil, RuleVerdict{ConfirmOnlyDropped: 1}},
		{"same-line Go finding confirmed", []scanner.Finding{goFinding}, 3, []scanner.Finding{goFinding}, RuleVerdict{Confirmed: 1}},
		{"unconfirmed Go finding dropped", []scanner.Finding{goFinding}, 0, nil, RuleVerdict{GoDropped: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := &Result{Rules: []string{rule}}
			if test.firLine != 0 {
				result.Findings = []scanner.Finding{{File: file, Line: test.firLine, Rule: rule, Message: "FIR"}}
			}
			got, stats := ApplyVerdict(VerdictInput{Go: test.goFindings, FIR: result, Requested: []string{file}})
			if !reflect.DeepEqual(describe(got), describe(test.want)) || *stats.Rules[rule] != test.wantStats {
				t.Fatalf("findings = %v, stats = %+v; want %v, %+v", got, *stats.Rules[rule], test.want, test.wantStats)
			}
		})
	}
}

func TestUnspecifiedImmutableFlagConfirmOnlyKeepsGoSeverity(t *testing.T) {
	const rule = "UnspecifiedImmutableFlag"
	for _, severity := range []string{"warning", "error"} {
		t.Run(severity, func(t *testing.T) {
			goFinding := scanner.Finding{File: "kotlin/A.kt", Line: 3, Rule: rule, Severity: severity, Message: "Go"}
			absolute, err := filepath.Abs(goFinding.File)
			if err != nil {
				t.Fatal(err)
			}
			firFinding := scanner.Finding{File: absolute, Line: 3, Rule: rule, Severity: "error", Message: "FIR"}
			got, stats := ApplyVerdict(VerdictInput{
				Go:        []scanner.Finding{goFinding},
				FIR:       &Result{Rules: []string{rule}, Findings: []scanner.Finding{firFinding}},
				Requested: []string{firFinding.File}, DisplayPath: map[string]string{firFinding.File: goFinding.File},
			})
			if !reflect.DeepEqual(got, []scanner.Finding{goFinding}) || stats.Rules[rule].Confirmed != 1 {
				t.Fatalf("got %v, stats %+v; want unchanged Go finding %v", got, stats, goFinding)
			}
		})
	}
}

func TestApplyVerdictConfirmOnlyFallbacksKeepGo(t *testing.T) {
	const file = "/src/Test.kt"
	for _, rule := range []string{"TestWithoutAssertion", "OptInMarkerExposedPublicly", "RunBlockingInTest", "UnspecifiedImmutableFlag"} {
		t.Run(rule, func(t *testing.T) {
			goFindings := []scanner.Finding{{File: file, Line: 3, Rule: rule}}
			for _, test := range []struct {
				name   string
				result *Result
			}{
				{"no FIR result", nil},
				{"compiler error", &Result{Rules: []string{rule}, ErrorFiles: map[string]string{file: "unresolved"}}},
				{"crash", &Result{Rules: []string{rule}, Crashed: map[string]string{file: "crashed"}}},
				{"rule error", &Result{Rules: []string{rule}, RuleErrors: map[string]map[string]string{rule: {file: "threw"}}}},
				{"unadvertised rule", &Result{Rules: []string{verdictRule}}},
			} {
				t.Run(test.name, func(t *testing.T) {
					got, _ := ApplyVerdict(VerdictInput{Go: goFindings, FIR: test.result, Requested: []string{file}})
					if !reflect.DeepEqual(got, goFindings) {
						t.Fatalf("got %v, want Go fallback %v", got, goFindings)
					}
				})
			}
		})
	}
}

func TestApplyVerdictConfirmOnlyMixedRulesAndFiles(t *testing.T) {
	const file = "/src/Test.kt"
	const otherFile = "/src/OtherTest.kt"
	const rule = "TestWithoutAssertion"
	goFinding := scanner.Finding{File: otherFile, Line: 3, Rule: rule}
	normal := scanner.Finding{File: file, Line: 3, Rule: verdictRule}
	got, stats := ApplyVerdict(VerdictInput{
		Go: []scanner.Finding{goFinding},
		FIR: &Result{Rules: []string{rule, verdictRule}, Findings: []scanner.Finding{
			{File: file, Line: 3, Rule: rule}, normal,
		}},
		Requested: []string{file, otherFile},
	})
	if !reflect.DeepEqual(describe(got), describe([]scanner.Finding{normal})) {
		t.Fatalf("got %v, want normal FIR-only finding %v", got, normal)
	}
	if stats.Rules[rule].ConfirmOnlyDropped != 1 || stats.Rules[rule].GoDropped != 1 || stats.Rules[verdictRule].FirAdded != 1 {
		t.Fatalf("stats = %+v, %+v", *stats.Rules[rule], *stats.Rules[verdictRule])
	}
}

func TestRunPassConfirmOnly(t *testing.T) {
	const file = "/src/Test.kt"
	const rule = "TestWithoutAssertion"
	checker := NewFakeFirChecker()
	checker.Rules = []string{rule}
	checker.Findings = []scanner.Finding{{File: file, Line: 3, Rule: rule}}
	var verbose strings.Builder
	got := RunPass(PassOptions{Enabled: true, Checker: checker, ActiveRules: []*api.Rule{{ID: rule}}, KotlinPaths: []string{file}, Verbose: true, VerboseOut: &verbose}, nil)
	if len(got) != 0 {
		t.Fatalf("shared pass added FIR-only findings: %v", got)
	}
	if !strings.Contains(verbose.String(), "confirm-only-dropped=1") {
		t.Fatalf("verbose output omitted dropped FIR-only finding: %s", verbose.String())
	}
}

func TestApplyVerdictConfirmOnlyPreservesGoFormAndMultiplicity(t *testing.T) {
	const rule = "TestWithoutAssertion"
	file := "Test.kt"
	absolute, err := filepath.Abs(file)
	if err != nil {
		t.Fatal(err)
	}
	goFinding := scanner.Finding{
		File: file, Line: 3, Rule: rule, Message: "Go", StartByte: 10, EndByte: 20,
		Fix: &scanner.Fix{},
	}
	firFinding := scanner.Finding{File: absolute, Line: 3, Rule: rule, Message: "FIR", StartByte: 30, EndByte: 40}
	otherFIRFinding := firFinding
	otherFIRFinding.Col = 2
	got, stats := ApplyVerdict(VerdictInput{
		Go: []scanner.Finding{goFinding},
		FIR: &Result{Rules: []string{rule}, Findings: []scanner.Finding{
			firFinding, otherFIRFinding,
		}},
		Requested: []string{absolute},
	})
	if !reflect.DeepEqual(got, []scanner.Finding{goFinding}) {
		t.Fatalf("got %v, want original Go finding %v", got, goFinding)
	}
	want := RuleVerdict{Confirmed: 1, EnrichedWithFix: 1, ConfirmOnlyDropped: 1}
	if *stats.Rules[rule] != want {
		t.Fatalf("stats = %+v; want %+v", *stats.Rules[rule], want)
	}
}
