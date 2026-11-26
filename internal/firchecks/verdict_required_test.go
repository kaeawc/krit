package firchecks

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

// A FIR-required rule's Go findings are reported only where FIR's verdict is
// authoritative: in a requested file FIR gated, or where its checker threw,
// they are withheld. Other rules, rules krit-fir does not advertise, and
// files FIR was never sent keep Go's findings (#1675).
func TestApplyVerdictWithholdsFIRRequiredRulesWithoutAVerdict(t *testing.T) {
	const (
		clean    = "/src/Clean.kt"
		gated    = "/src/Gated.kt"
		threw    = "/src/Threw.kt"
		excluded = "/build.gradle.kts"
		required = "UselessElvisOnNonNull"
		other    = "UnsafeCallOnNullableType"
		absent   = "UnnecessarySafeCall" // required, but not advertised by krit-fir
	)
	at := func(file, rule string, line int) scanner.Finding {
		return scanner.Finding{File: file, Line: line, Rule: rule, Message: "go"}
	}
	goFindings := []scanner.Finding{
		at(clean, required, 1),
		at(gated, required, 2),
		at(gated, other, 3),
		at(gated, absent, 4),
		at(threw, required, 5),
		at(excluded, required, 6),
	}
	result := &Result{
		Rules:      []string{required, other},
		Findings:   []scanner.Finding{{File: clean, Line: 1, Rule: required, Message: "fir"}},
		ErrorFiles: map[string]string{gated: "Unresolved reference 'serializer'."},
		RuleErrors: map[string]map[string]string{required: {threw: "IllegalStateException"}},
	}
	in := VerdictInput{Go: goFindings, FIR: result, Requested: []string{clean, gated, threw},
		RequireAuthoritative: map[string]bool{required: true, absent: true}}
	got, stats := ApplyVerdict(in)
	want := []scanner.Finding{at(gated, other, 3), at(gated, absent, 4), at(excluded, required, 6), at(clean, required, 1)}
	if !reflect.DeepEqual(describe(got), describe(want)) {
		t.Fatalf("findings\ngot:  %v\nwant: %v", describe(got), describe(want))
	}
	if stats.Withheld != 2 || stats.Rules[required].Withheld != 2 || stats.Rules[other].Withheld != 0 {
		t.Fatalf("withheld = %d (rule %d, other %d); want 2, 2, 0", stats.Withheld, stats.Rules[required].Withheld, stats.Rules[other].Withheld)
	}

	in.RequireAuthoritative = nil
	if got, stats := ApplyVerdict(in); len(got) != len(goFindings) || stats.Withheld != 0 {
		t.Fatalf("without the policy every Go finding outside FIR's verdict stands; got %v", describe(got))
	}
}

// When FIR gates every requested file, a FIR-required rule still has no
// verdict anywhere, so its Go findings are withheld.
func TestApplyVerdictWithholdsWhenEveryFileIsGated(t *testing.T) {
	const file, rule = "/src/Gated.kt", "UnsafeCast"
	got, stats := ApplyVerdict(VerdictInput{
		Go:                   []scanner.Finding{{File: file, Line: 1, Rule: rule}, {File: file, Line: 2, Rule: "Other"}},
		FIR:                  &Result{Rules: []string{rule}, ErrorFiles: map[string]string{file: "Unresolved reference 'x'."}},
		Requested:            []string{file},
		RequireAuthoritative: map[string]bool{rule: true},
	})
	if len(got) != 1 || got[0].Rule != "Other" || stats.Withheld != 1 {
		t.Fatalf("got %v, withheld %d; want only the Other finding kept", describe(got), stats.Withheld)
	}
}

func TestRequiredRulesDefaultAndConfig(t *testing.T) {
	if got := RequiredRules(nil); len(got) != len(DefaultRequiredRules) || !got["UselessElvisOnNonNull"] {
		t.Fatalf("default = %v", got)
	}
	for _, test := range []struct {
		name string
		fir  map[string]interface{}
		want map[string]bool
	}{
		{"configured", map[string]interface{}{"requireAuthoritativeRules": []interface{}{"UnsafeCast"}}, map[string]bool{"UnsafeCast": true}},
		{"declared empty turns it off", map[string]interface{}{"requireAuthoritativeRules": []interface{}{}}, map[string]bool{}},
		{"absent keeps the default", map[string]interface{}{"goAuthoritativeRules": []interface{}{"UnsafeCast"}}, nil},
	} {
		got := RequiredRules(config.NewConfigFromData(map[string]interface{}{"fir": test.fir}))
		if test.want == nil {
			if len(got) != len(DefaultRequiredRules) {
				t.Errorf("%s: got %v, want the default", test.name, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

// The verbose summary and the default coverage line report what was withheld.
func TestWithheldFindingsAreReported(t *testing.T) {
	stats := VerdictStats{
		Rules:      map[string]*RuleVerdict{"UnsafeCast": {Withheld: 3}},
		GatedFiles: map[string]string{"/src/A.kt": "Unresolved reference 'x'."},
		Withheld:   3,
	}
	var verbose strings.Builder
	writeVerdictSummary(&verbose, stats)
	for _, line := range []string{
		"verbose: FIR verdict UnsafeCast: confirmed=0 go-dropped=0 fir-added=0 enriched-with-fix=0 suppressed=0 files-gated=1 confirm-only-dropped=0 rule-errors=0 withheld=3",
		"verbose: FIR verdict: 3 Go findings withheld in gated files for FIR-required rules (fir.requireAuthoritativeRules)",
	} {
		if !strings.Contains(verbose.String(), line) {
			t.Fatalf("verbose summary lacks %q:\n%s", line, verbose.String())
		}
	}
	line := output.FirCoverageLine(Coverage(stats, nil))
	if !strings.Contains(line, "so Go-only findings stand for them, except 3 withheld for FIR-required rules") {
		t.Fatalf("coverage line %q", line)
	}
}
