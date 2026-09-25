package firchecks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const throwingRule = "ThrowingRule"

// ruleErrorFixture is two authoritative files and two advertised rules where
// throwingRule's checker threw on A.kt only. FIR reported a partial finding
// for throwingRule on A.kt before throwing.
type ruleErrorFixture struct {
	a, b   string
	goIn   []scanner.Finding
	fir    []scanner.Finding
	errors map[string]map[string]string
}

func newRuleErrorFixture() ruleErrorFixture {
	const a, b = "/src/A.kt", "/src/B.kt"
	return ruleErrorFixture{
		a: a, b: b,
		goIn: []scanner.Finding{
			{File: a, Line: 2, StartByte: 10, EndByte: 20, Rule: throwingRule, Message: "go: kept, checker threw here"},
			{File: a, Line: 3, StartByte: 30, EndByte: 40, Rule: verdictRule, Message: "go: dropped, FIR clean"},
			{File: b, Line: 2, StartByte: 10, EndByte: 20, Rule: throwingRule, Message: "go: dropped, FIR clean on B"},
		},
		fir: []scanner.Finding{
			{File: a, Line: 5, StartByte: 50, EndByte: 55, Rule: throwingRule, Message: "fir: partial, discarded"},
			{File: a, Line: 6, StartByte: 60, EndByte: 65, Rule: verdictRule, Message: "fir: added on A"},
			{File: b, Line: 7, StartByte: 70, EndByte: 75, Rule: throwingRule, Message: "fir: added on B"},
		},
		errors: map[string]map[string]string{throwingRule: {a: "krit-fir: checker threw java.lang.IllegalStateException: boom"}},
	}
}

func (fx ruleErrorFixture) wantMessages() []string {
	return []string{
		"fir: added on A",
		"fir: added on B",
		"go: kept, checker threw here",
	}
}

func messages(findings []scanner.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Message)
	}
	slices.Sort(out)
	return out
}

// A checker exception makes only that (file, rule) pair non-authoritative:
// Go's findings for the rule on that file stand and FIR's (possibly partial)
// ones are dropped, while the same rule on other files and other rules on
// the same file keep the FIR verdict.
func TestApplyVerdictRuleErrorFallsBackForThatRuleAndFileOnly(t *testing.T) {
	fx := newRuleErrorFixture()
	got, stats := ApplyVerdict(VerdictInput{
		Go:        slices.Clone(fx.goIn),
		FIR:       &Result{Findings: fx.fir, Rules: []string{throwingRule, verdictRule}, RuleErrors: fx.errors},
		Requested: []string{fx.a, fx.b},
	})
	if d, want := messages(got), fx.wantMessages(); !reflect.DeepEqual(d, want) {
		t.Fatalf("got %v, want %v", d, want)
	}
	if stats.AuthoritativeFiles != 2 || len(stats.GatedFiles) != 0 {
		t.Fatalf("a rule error must not gate the file: authoritative=%d gated=%v", stats.AuthoritativeFiles, stats.GatedFiles)
	}
	want := []RuleError{{File: fx.a, Rule: throwingRule, Message: fx.errors[throwingRule][fx.a]}}
	if !reflect.DeepEqual(stats.RuleErrors, want) {
		t.Fatalf("RuleErrors = %+v, want %+v", stats.RuleErrors, want)
	}
	if r := stats.Rules[throwingRule]; r.RuleErrorFiles != 1 || r.GoDropped != 1 || r.FirAdded != 1 {
		t.Fatalf("%s stats = %+v", throwingRule, *r)
	}
	if r := stats.Rules[verdictRule]; r.RuleErrorFiles != 0 || r.GoDropped != 1 || r.FirAdded != 1 {
		t.Fatalf("%s stats = %+v", verdictRule, *r)
	}
}

// Rule errors on files or rules FIR does not own anyway are not counted.
func TestApplyVerdictIgnoresRuleErrorsOutsideTheVerdict(t *testing.T) {
	const a, gated = "/src/A.kt", "/src/Gated.kt"
	_, stats := ApplyVerdict(VerdictInput{
		FIR: &Result{
			Rules:      []string{verdictRule},
			ErrorFiles: map[string]string{gated: "Unresolved reference 'x'."},
			RuleErrors: map[string]map[string]string{
				verdictRule:  {gated: "threw on a gated file", "/src/Unrequested.kt": "threw on an unrequested file"},
				"NotAdvised": {a: "threw for an unadvertised rule"},
			},
		},
		Requested: []string{a, gated},
	})
	if len(stats.RuleErrors) != 0 || stats.Rules[verdictRule].RuleErrorFiles != 0 {
		t.Fatalf("RuleErrors = %+v, rule stats = %+v", stats.RuleErrors, *stats.Rules[verdictRule])
	}
}

// The wire shape krit-fir emits reaches Result.RuleErrors.
func TestCheckResponseRuleErrorsReachResult(t *testing.T) {
	line := `{"id":1,"succeeded":2,"skipped":0,"findings":[],"rules":["ThrowingRule"],"crashed":{},"errorFiles":{},` +
		`"ruleErrors":{"ThrowingRule":{"/src/A.kt":"krit-fir: checker threw java.lang.IllegalStateException: probe \"boom\" \\ on purpose"}}}`
	var resp CheckResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatal(err)
	}
	res := newResult()
	res.addResponse(&resp)
	want := map[string]map[string]string{throwingRule: {"/src/A.kt": `krit-fir: checker threw java.lang.IllegalStateException: probe "boom" \ on purpose`}}
	if !reflect.DeepEqual(res.RuleErrors, want) {
		t.Fatalf("RuleErrors = %v, want %v", res.RuleErrors, want)
	}
}

// A rule error is cached with the file's entry, so an all-hit warm run
// applies the same verdict as the cold run that wrote it.
func TestCachedRuleErrorsReplayTheColdVerdict(t *testing.T) {
	dir := t.TempDir()
	fx := newRuleErrorFixture()
	fx.a, fx.b = filepath.Join(dir, "A.kt"), filepath.Join(dir, "B.kt")
	for i := range fx.goIn {
		fx.goIn[i].File = filepath.Join(dir, filepath.Base(fx.goIn[i].File))
	}
	for i := range fx.fir {
		fx.fir[i].File = filepath.Join(dir, filepath.Base(fx.fir[i].File))
	}
	fx.errors = map[string]map[string]string{throwingRule: {fx.a: "krit-fir: checker threw boom"}}
	for _, p := range []string{fx.a, fx.b} {
		if err := os.WriteFile(p, []byte(strings.Repeat("val x = 1\n", 10)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	resp := &CheckResponse{Rules: []string{throwingRule, verdictRule}, RuleErrors: fx.errors}
	for _, f := range fx.fir {
		resp.Findings = append(resp.Findings, FirFinding{Path: f.File, Line: f.Line, Col: 1, StartByte: f.StartByte, EndByte: f.EndByte, Rule: f.Rule, Message: f.Message})
	}
	cold := newResult()
	cold.addResponse(resp)

	cacheDir, _ := CacheDir(t.TempDir())
	files := []string{fx.a, fx.b}
	if n := WriteFreshEntriesForFingerprint(cacheDir, files, resp, "fp"); n != 2 {
		t.Fatalf("wrote %d entries, want 2", n)
	}
	hits, misses := ClassifyFilesForFingerprint(cacheDir, files, "fp")
	if len(hits) != 2 || len(misses) != 0 {
		t.Fatalf("hits=%d misses=%v, want both files to hit", len(hits), misses)
	}
	warm := assembleFromCache(hits)
	if !reflect.DeepEqual(warm.RuleErrors, fx.errors) {
		t.Fatalf("cached RuleErrors = %v, want %v", warm.RuleErrors, fx.errors)
	}
	run := func(r *Result) []string {
		got, _ := ApplyVerdict(VerdictInput{Go: slices.Clone(fx.goIn), FIR: r, Requested: files})
		return messages(got)
	}
	if c, w := run(cold), run(warm); !reflect.DeepEqual(c, w) || !reflect.DeepEqual(c, fx.wantMessages()) {
		t.Fatalf("cold %v, warm %v, want %v", c, w, fx.wantMessages())
	}
}

// -v names each rule error and counts them in the summary and per rule.
func TestRunPassReportsRuleErrorsVerbosely(t *testing.T) {
	p := newVerdictProject(t, map[string]string{
		"A.kt": "package a\n\nclass A { fun f() = run(Dispatchers.IO) }\n",
		"B.kt": "package b\n\nclass B { fun f() = run(Dispatchers.IO) }\n",
	}, nil)
	goA := p.at("A.kt", "Dispatchers.IO", verdictRule, "go: kept, checker threw on A")
	goB := p.at("B.kt", "Dispatchers.IO", verdictRule, "go: dropped, FIR clean on B")
	checker := NewFakeFirChecker()
	checker.Rules = []string{verdictRule}
	checker.RuleErrors = map[string]map[string]string{verdictRule: {p.abs("A.kt"): "krit-fir: checker threw java.lang.IllegalStateException: boom\nmore"}}
	var verbose strings.Builder
	got := RunPass(PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule, Category: "coroutines"}},
		ParsedFiles: p.files(),
		KotlinPaths: []string{"A.kt", "B.kt"},
		Tracker:     perf.New(false),
		Verbose:     true,
		VerboseOut:  &verbose,
	}, []scanner.Finding{goA, goB})
	if d := messages(got); !reflect.DeepEqual(d, []string{"go: kept, checker threw on A"}) {
		t.Fatalf("got %v", d)
	}
	out := verbose.String()
	for _, want := range []string{
		"2 authoritative files, 0 gated (compiler error or crash), 0 excluded (scripts or not in a JVM source set), 1 rule errors",
		"verbose: FIR rule error: " + verdictRule + ": " + p.abs("A.kt") + ": krit-fir: checker threw java.lang.IllegalStateException: boom\n",
		"rule-errors=1\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("verbose output missing %q:\n%s", want, out)
		}
	}
}
