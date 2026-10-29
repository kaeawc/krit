package firchecks

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestIncrementalFIRDiagnostic(t *testing.T) {
	gated := map[string]string{
		"Broken.kt":        "Unresolved reference 'missing'.",
		"Other.kt":         "Type mismatch.",
		"build.gradle.kts": "krit-fir: not compiled; Kotlin scripts are not part of the module compilation",
	}
	for _, test := range []struct {
		name        string
		result      *Result
		noCache     bool
		environment string
		want        string
	}{
		{name: "nil response"},
		{name: "all files served from cache", result: &Result{ErrorFiles: gated}},
		{name: "compiler errors", result: &Result{ErrorFiles: gated, incrementalErrorFiles: 2, Incremental: &IncrementalPlan{Path: "full", Reason: "cold"}},
			want: "incremental FIR off: 2 requested files gated by compiler errors, no snapshot"},
		{name: "older helper without planner telemetry", result: &Result{responseApplied: true, ErrorFiles: gated, incrementalErrorFiles: 2},
			want: "incremental FIR: 2 requested files gated by compiler errors (planner telemetry unavailable)"},
		{name: "crash is not compiler error", result: &Result{Crashed: map[string]string{"Broken.kt": "crash"}}},
		{name: "exclusion is not compiler error", result: &Result{ErrorFiles: map[string]string{
			"Other.kt": "krit-fir: not compiled; the file's source set is not part of the JVM compilation"}}},
		{name: "incremental despite gated files", result: &Result{ErrorFiles: gated, Incremental: &IncrementalPlan{Path: "incremental"}}},
		{name: "cold compile", result: &Result{Incremental: &IncrementalPlan{Path: "full", Reason: "cold"}}, want: "incremental FIR: full (cold)"},
		{name: "cached gates do not describe current compile", result: &Result{ErrorFiles: gated, Incremental: &IncrementalPlan{Path: "full", Reason: "cold"}}, want: "incremental FIR: full (cold)"},
		{name: "module fallback detail", result: &Result{Incremental: &IncrementalPlan{Path: "full", Reason: "slice_unavailable", Detail: "COMPILE_ERRORS: compile errors in 6 files"}},
			want: "incremental FIR: full (slice_unavailable: COMPILE_ERRORS: compile errors in 6 files)"},
		{name: "tiered rules", result: &Result{ErrorFiles: gated, Incremental: &IncrementalPlan{Path: "full", Reason: "tiered_rules"}},
			want: "incremental FIR: full (tiered_rules)"},
		{name: "environment zero", result: &Result{ErrorFiles: gated}, environment: "0", want: "incremental FIR off: KRIT_FIR_INCREMENTAL=0"},
		{name: "environment false", result: &Result{}, environment: " FALSE ", want: "incremental FIR off: KRIT_FIR_INCREMENTAL=FALSE"},
		{name: "environment off", result: &Result{}, environment: "off", want: "incremental FIR off: KRIT_FIR_INCREMENTAL=off"},
		{name: "environment enabled", result: &Result{}, environment: "1"},
		{name: "response takes precedence", result: &Result{Incremental: &IncrementalPlan{Path: "incremental"}}, environment: "0"},
		{name: "no cache", result: &Result{}, noCache: true, want: "incremental FIR snapshot persistence off: --no-cache"},
		{name: "no cache still incremental", result: &Result{Incremental: &IncrementalPlan{Path: "incremental"}}, noCache: true,
			want: "incremental FIR snapshot persistence off: --no-cache"},
		{name: "no cache with errors", result: &Result{ErrorFiles: gated, incrementalErrorFiles: 2, Incremental: &IncrementalPlan{Path: "full", Reason: "cold"}}, noCache: true,
			want: "incremental FIR off: 2 requested files gated by compiler errors, no snapshot; incremental FIR snapshot persistence off: --no-cache"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := incrementalFIRDiagnostic(test.result, test.noCache, test.environment); got != test.want {
				t.Fatalf("diagnostic = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPassProductionFirCheckerNoCacheDiagnostic(t *testing.T) {
	t.Setenv("KRIT_FIR_INCREMENTAL", "1")
	project := newVerdictProject(t, map[string]string{"Sample.kt": "fun sample() {}\n"}, nil)
	old := runMissesForCache
	t.Cleanup(func() { runMissesForCache = old })
	runMissesForCache = func(_ string, _ []string, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, targets ...CompileTarget) (*CheckResponse, error) {
		if len(targets) != 1 || targets[0].IncrementalDir != "" {
			t.Fatalf("IncrementalDir target = %v, want one target with no persistence directory", targets)
		}
		return &CheckResponse{Incremental: &IncrementalPlan{Path: "full", Reason: "cold"}}, nil
	}
	var output bytes.Buffer
	checker := &ProductionFirChecker{RepoDir: project.t.TempDir(), NoCache: true}
	_, err := StartPass(context.Background(), PassOptions{
		Enabled: true, Checker: checker, ActiveRules: []*api.Rule{{ID: verdictRule}},
		KotlinPaths: []string{project.abs("Sample.kt")}, Verbose: true, VerboseOut: &output,
	}).Finish(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "incremental FIR snapshot persistence off: --no-cache") {
		t.Fatalf("missing no-cache diagnostic: %s", output.String())
	}
}

func TestPassIncrementalFIRDiagnostic(t *testing.T) {
	t.Setenv("KRIT_FIR_INCREMENTAL", "1")
	project := newVerdictProject(t, map[string]string{"Broken.kt": "fun broken() = missing()\n"}, nil)
	result := newResult()
	result.addResponse(&CheckResponse{
		Rules: []string{verdictRule}, ErrorFiles: map[string]string{project.abs("Broken.kt"): "Unresolved reference 'missing'."},
		Incremental: &IncrementalPlan{Path: "full", Reason: "cold"},
	})
	base := []scanner.Finding{project.at("Broken.kt", "missing", verdictRule, "Go finding")}
	for _, verbose := range []bool{false, true} {
		var output bytes.Buffer
		tracker := perf.New(true)
		checker := checkerFunc(func(_, _, _, _ []string, _ RuleConfigs, _ FileFacts) (*Result, error) {
			return result, nil
		})
		got, err := StartPass(context.Background(), PassOptions{
			Enabled: true, Checker: checker, ActiveRules: []*api.Rule{{ID: verdictRule}},
			KotlinPaths: []string{project.abs("Broken.kt")}, Verbose: verbose, VerboseOut: &output, Tracker: tracker,
		}).Finish(base)
		if err != nil || !reflect.DeepEqual(got, base) {
			t.Fatalf("findings = %v, err = %v; want %v", got, err, base)
		}
		message := "incremental FIR off: 1 requested file gated by compiler errors, no snapshot"
		wantCount := 0
		if verbose {
			wantCount = 1
		}
		if count := strings.Count(output.String(), message); count != wantCount {
			t.Fatalf("verbose diagnostic count = %d, want %d: %s", count, wantCount, output.String())
		}
		var timing bytes.Buffer
		if err := perf.RenderTree(&timing, tracker.GetTimings(), perf.RenderOptions{}); err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(timing.String(), message); count != 1 {
			t.Fatalf("perf diagnostic count = %d, want 1: %s", count, timing.String())
		}
	}
}

func TestIncrementalFIRResponseErrorCount(t *testing.T) {
	result := newResult()
	result.ErrorFiles["Cached.kt"] = "Unresolved reference 'old'."
	result.addResponse(&CheckResponse{
		ErrorFiles: map[string]string{
			"Broken.kt":   "Unresolved reference 'missing'.",
			"Excluded.kt": "krit-fir: not compiled; the file's source set is not part of the JVM compilation",
		},
		Crashed:     map[string]string{"Crashed.kt": "checker failed"},
		Incremental: &IncrementalPlan{Path: "full", Reason: "cold"},
	})
	if got, want := incrementalFIRDiagnostic(result, false, ""), "incremental FIR off: 1 requested file gated by compiler errors, no snapshot"; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
	result.addResponse(&CheckResponse{Incremental: &IncrementalPlan{Path: "full", Reason: "cold"}})
	if got, want := incrementalFIRDiagnostic(result, false, ""), "incremental FIR: full (cold)"; got != want {
		t.Fatalf("diagnostic after clean compile = %q, want %q", got, want)
	}
}

func TestIncrementalFIRResponseErrorCountWithoutPlan(t *testing.T) {
	result := newResult()
	result.ErrorFiles["Cached.kt"] = "Unresolved reference 'old'."
	result.addResponse(&CheckResponse{ErrorFiles: map[string]string{
		"Broken.kt":   "Unresolved reference 'missing'.",
		"Excluded.kt": "krit-fir: not compiled; the file's source set is not part of the JVM compilation",
	}})
	if got, want := incrementalFIRDiagnostic(result, false, ""), "incremental FIR: 1 requested file gated by compiler errors (planner telemetry unavailable)"; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}
