package firchecks

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	isolateTestFile    = "src/test/kotlin/Fake.kt"
	isolateAndroidFile = "src/androidTest/kotlin/Fake.kt"
	isolateMainFile    = "src/main/kotlin/Main.kt"
)

// isolationProject has the same class in test and androidTest, which a merged
// compile reports as a redeclaration in both files.
func isolationProject(t *testing.T) *verdictProject {
	return newVerdictProject(t, map[string]string{
		isolateTestFile:    "package app\n\nclass Fake { fun f() = run(Dispatchers.IO) }\n",
		isolateAndroidFile: "package app\n\nclass Fake { fun f() = run(Dispatchers.IO) }\n",
		isolateMainFile:    "package app\n\nclass Main { fun f() = run(Dispatchers.IO) }\n",
	}, nil)
}

func isolationOptions(p *verdictProject, checker FirChecker, isolator SourceSetIsolator, verbose *strings.Builder) PassOptions {
	return PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule, Category: "coroutines"}},
		ParsedFiles: p.files(),
		KotlinPaths: []string{isolateAndroidFile, isolateMainFile, isolateTestFile},
		Tracker:     perf.New(false),
		Verbose:     verbose != nil,
		VerboseOut:  verbose,
		Isolator:    isolator,
	}
}

func isolationGoFindings(p *verdictProject) []scanner.Finding {
	return []scanner.Finding{
		p.at(isolateTestFile, "Dispatchers.IO", verdictRule, "go: test"),
		p.at(isolateAndroidFile, "Dispatchers.IO", verdictRule, "go: androidTest"),
		p.at(isolateMainFile, "Dispatchers.IO", verdictRule, "go: main"),
	}
}

func redeclarationChecker(p *verdictProject) *FakeFirChecker {
	checker := NewFakeFirChecker()
	checker.Rules = []string{verdictRule}
	checker.ErrorFiles = map[string]string{
		p.abs(isolateTestFile):    "Redeclaration:\nclass Fake : Any",
		p.abs(isolateAndroidFile): "Redeclaration:\nclass Fake : Any",
	}
	// The main file is not gated, so its verdict is FIR's: no finding.
	return checker
}

func TestPassRechecksFilesGatedByACrossSourceSetRedeclaration(t *testing.T) {
	p := isolationProject(t)
	var gotGated []string
	var gotRequest struct {
		ID    int64 `json:"id"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
		Rules []string `json:"rules"`
	}
	isolator := func(_ context.Context, _ string, gated []string, request []byte) ([]byte, []string, error) {
		gotGated = gated
		if err := json.Unmarshal(request, &gotRequest); err != nil {
			t.Fatal(err)
		}
		// Compiled on its own, each set is clean; FIR keeps the test set's finding.
		finding := p.fir(isolateTestFile, "Dispatchers.IO", verdictRule)
		resp, _ := json.Marshal(map[string]any{
			"id":    gotRequest.ID,
			"rules": []string{verdictRule},
			"findings": []FirFinding{{Path: finding.File, Line: finding.Line, Col: finding.Col,
				StartByte: finding.StartByte, EndByte: finding.EndByte, Rule: verdictRule, Message: finding.Message, Confidence: 1}},
		})
		return resp, []string{p.abs(isolateAndroidFile), p.abs(isolateTestFile)}, nil
	}
	var verbose strings.Builder
	got := RunPass(isolationOptions(p, redeclarationChecker(p), isolator, &verbose), isolationGoFindings(p))

	if want := []string{p.abs(isolateAndroidFile), p.abs(isolateTestFile)}; !reflect.DeepEqual(gotGated, want) {
		t.Fatalf("isolator should receive the redeclaration-gated files, got %v want %v", gotGated, want)
	}
	if len(gotRequest.Files) != 2 || !slices.Equal(gotRequest.Rules, []string{verdictRule}) {
		t.Fatalf("the recheck request should carry the gated files and the pass's rules: %+v", gotRequest)
	}
	want := describe([]scanner.Finding{p.at(isolateTestFile, "Dispatchers.IO", verdictRule, "go: test")})
	if d := describe(got); !reflect.DeepEqual(d, want) {
		t.Fatalf("rechecked files should get FIR's verdict\ngot:  %v\nwant: %v", d, want)
	}
	if !strings.Contains(verbose.String(), "3 authoritative files, 0 gated") ||
		!strings.Contains(verbose.String(), "2 files gated by duplicate declarations; rechecked 2 files") {
		t.Fatalf("verbose output should report the recheck:\n%s", verbose.String())
	}
}

// A duplicate declaration gates its callers too, on another error (an
// ambiguous call, an unresolved reference). The recheck checks every gated
// file, not just the duplicates, so the callers get their verdict back
// (#1661); a file the merged check answered is not checked again.
func TestPassRechecksTheCallersADuplicateDeclarationGated(t *testing.T) {
	p := isolationProject(t)
	checker := redeclarationChecker(p)
	checker.ErrorFiles[p.abs(isolateMainFile)] = "Overload resolution ambiguity between candidates:\nfun f(): Unit"
	var requested []string
	isolator := func(_ context.Context, _ string, _ []string, request []byte) ([]byte, []string, error) {
		var req struct {
			ID    int64                   `json:"id"`
			Files []struct{ Path string } `json:"files"`
		}
		if err := json.Unmarshal(request, &req); err != nil {
			t.Fatal(err)
		}
		for _, f := range req.Files {
			requested = append(requested, f.Path)
		}
		resp, _ := json.Marshal(map[string]any{"id": req.ID, "rules": []string{verdictRule}, "findings": []FirFinding{}})
		return resp, requested, nil
	}
	var verbose strings.Builder
	RunPass(isolationOptions(p, checker, isolator, &verbose), isolationGoFindings(p))
	slices.Sort(requested)
	want := []string{p.abs(isolateAndroidFile), p.abs(isolateMainFile), p.abs(isolateTestFile)}
	slices.Sort(want)
	if !slices.Equal(requested, want) {
		t.Fatalf("the recheck should check every gated file, the ambiguous caller included: got %v want %v", requested, want)
	}
	if !strings.Contains(verbose.String(), "replaced 3 gated verdicts") {
		t.Fatalf("the caller's verdict should be replaced too:\n%s", verbose.String())
	}

	// With the caller answered by the merged check, only the duplicates are checked again.
	requested = nil
	RunPass(isolationOptions(p, redeclarationChecker(p), isolator, nil), isolationGoFindings(p))
	if len(requested) != 2 || slices.Contains(requested, p.abs(isolateMainFile)) {
		t.Fatalf("an answered file should not be checked again: %v", requested)
	}
}

func TestPassKeepsMergedVerdictWhenTheRecheckFails(t *testing.T) {
	p := isolationProject(t)
	for name, isolator := range map[string]SourceSetIsolator{
		"error": func(context.Context, string, []string, []byte) ([]byte, []string, error) {
			return nil, nil, errors.New("worker failed")
		},
		"no project qualified": func(context.Context, string, []string, []byte) ([]byte, []string, error) {
			return nil, nil, nil
		},
		"no checkers": func(_ context.Context, _ string, _ []string, request []byte) ([]byte, []string, error) {
			return []byte(`{"id":2,"rules":[]}`), []string{p.abs(isolateTestFile)}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := RunPass(isolationOptions(p, redeclarationChecker(p), isolator, nil), isolationGoFindings(p))
			want := describe(isolationGoFindings(p)[:2])
			if d := describe(got); !reflect.DeepEqual(d, want) {
				t.Fatalf("gated files should keep their Go findings\ngot:  %v\nwant: %v", d, want)
			}
		})
	}
}

func TestPassKeepsTheMergedVerdictOfFilesItAlreadyAnswered(t *testing.T) {
	p := isolationProject(t)
	// The recheck lacks some context for main: it crashes there and reports a stray finding.
	isolator := func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, []string, error) {
		main := p.fir(isolateMainFile, "Dispatchers.IO", verdictRule)
		resp, _ := json.Marshal(map[string]any{
			"id": isolateRequestID, "rules": []string{verdictRule},
			"findings": []FirFinding{{Path: main.File, Line: main.Line, Col: main.Col, Rule: verdictRule, Message: "stray", Confidence: 1}},
			"crashed":  map[string]string{main.File: "isolated compile crashed"},
		})
		return resp, []string{p.abs(isolateAndroidFile), p.abs(isolateMainFile), p.abs(isolateTestFile)}, nil
	}
	got := RunPass(isolationOptions(p, redeclarationChecker(p), isolator, nil), isolationGoFindings(p))
	// main keeps FIR's merged verdict (no finding); the two gated files take the clean recheck.
	if d := describe(got); len(d) != 0 {
		t.Fatalf("no finding should survive: %v", d)
	}
}

func TestPassDoesNotRecheckOtherCompilerErrors(t *testing.T) {
	p := isolationProject(t)
	checker := NewFakeFirChecker()
	checker.Rules = []string{verdictRule}
	checker.ErrorFiles = map[string]string{p.abs(isolateTestFile): "Unresolved reference 'missing'."}
	called := false
	isolator := func(context.Context, string, []string, []byte) ([]byte, []string, error) {
		called = true
		return nil, nil, nil
	}
	RunPass(isolationOptions(p, checker, isolator, nil), isolationGoFindings(p))
	if called {
		t.Fatal("only duplicate declarations should trigger an isolated recheck")
	}
}

func TestDuplicateDeclarationReasons(t *testing.T) {
	for reason, want := range map[string]bool{
		"Redeclaration:\nclass Fake : Any":          true,
		"Conflicting overloads:\nfun f(): Unit":     true,
		"Unresolved reference 'Redeclaration'.":     false,
		"krit-fir: not checked; Redeclaration: x":   false,
		"Overload resolution ambiguity between ...": false,
	} {
		if got := duplicateDeclaration(reason); got != want {
			t.Errorf("duplicateDeclaration(%q) = %t, want %t", reason, got, want)
		}
	}
}

func TestReplaceFilesKeepsOtherFilesAndIncrementalAccount(t *testing.T) {
	r := newResult()
	r.Findings = []scanner.Finding{{File: "/a.kt", Rule: "R"}, {File: "/b.kt", Rule: "R"}}
	r.ErrorFiles["/a.kt"] = "Redeclaration:\nclass A"
	r.ErrorFiles["/c.kt"] = "Unresolved reference 'x'."
	r.Crashed["/a.kt"] = "boom"
	r.addRuleError("R", "/a.kt", "threw")
	r.addRuleError("R", "/b.kt", "threw")
	r.Incremental = &IncrementalPlan{Path: "full"}
	r.incrementalErrorFiles = 2

	r.replaceFiles([]string{"/a.kt"}, &CheckResponse{
		Rules:      []string{"R", "S"},
		Findings:   []FirFinding{{Path: "/a.kt", Line: 1, Col: 1, Rule: "S"}, {Path: "/b.kt", Line: 1, Col: 1, Rule: "S"}},
		ErrorFiles: map[string]string{"/a.kt": "Unresolved reference 'y'."},
	})

	if len(r.Findings) != 2 || r.Findings[0].File != "/b.kt" || r.Findings[1].File != "/a.kt" || r.Findings[1].Rule != "S" {
		t.Fatalf("findings should be /b.kt's plus the recheck's /a.kt: %+v", r.Findings)
	}
	if r.ErrorFiles["/a.kt"] != "Unresolved reference 'y'." || r.ErrorFiles["/c.kt"] == "" || r.Crashed["/a.kt"] != "" {
		t.Fatalf("error files and crashes should be replaced for /a.kt only: %v %v", r.ErrorFiles, r.Crashed)
	}
	if _, ok := r.RuleErrors["R"]["/a.kt"]; ok || r.RuleErrors["R"]["/b.kt"] == "" {
		t.Fatalf("rule errors should be replaced for /a.kt only: %v", r.RuleErrors)
	}
	if !slices.Equal(r.Rules, []string{"R", "S"}) || r.Incremental.Path != "full" || r.incrementalErrorFiles != 2 {
		t.Fatalf("rules should union and the incremental account stay: %v %+v %d", r.Rules, r.Incremental, r.incrementalErrorFiles)
	}
}

func TestIsolationOutcomeListsFilesStillGated(t *testing.T) {
	var out strings.Builder
	writeIsolationOutcome(&out, isolationOutcome{gated: 2, rechecked: 5, replaced: 2, stillGated: 1, still: []string{"/p/A.kt: Redeclaration: class A"}})
	for _, line := range []string{
		"verbose: FIR source sets: 2 files gated by duplicate declarations; rechecked 5 files with each source set compiled on its own in 0s, replaced 2 gated verdicts, 1 still gated by one\n",
		"verbose: FIR source sets: still gated: /p/A.kt: Redeclaration: class A\n",
	} {
		if !strings.Contains(out.String(), line) {
			t.Fatalf("missing %q in:\n%s", line, out.String())
		}
	}
}
