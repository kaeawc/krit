package pipeline

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestCrossFilePhase_RecoversSerialRulePanics(t *testing.T) {
	panicRule := api.FakeRule("SerialPanic", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		ctx.Emit(scanner.Finding{File: "panic.kt", Line: 1, Message: "must be discarded"})
		panic("boom")
	}))
	in := DispatchResult{IndexResult: IndexResult{ParseResult: ParseResult{ActiveRules: []*api.Rule{panicRule}}}}
	result, err := (CrossFilePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("CrossFilePhase.Run: %v", err)
	}
	assertPanicRule(t, result.Stats.Errors, "SerialPanic")
	if got := result.Findings.Len(); got != 0 {
		t.Fatalf("Findings.Len = %d, want 0 after recovered panic; got %+v", got, result.Findings.Findings())
	}
}

func TestCrossFilePhase_RecoversModuleAwareRulePanics(t *testing.T) {
	panicRule := api.FakeRule("ModulePanic", api.WithNeeds(api.NeedsModuleIndex), api.WithCheck(func(ctx *api.Context) {
		ctx.Emit(scanner.Finding{File: "panic.kt", Line: 1, Message: "must be discarded"})
		panic("boom")
	}))
	result := CrossFileResult{}
	collector := scanner.NewFindingCollector(0)
	(CrossFilePhase{}).runModuleAwareRules(
		DispatchResult{IndexResult: IndexResult{ModuleIndex: &module.PerModuleIndex{}}},
		[]*api.Rule{panicRule}, collector, &result,
	)
	assertPanicRule(t, result.Stats.Errors, "ModulePanic")
	if got := collector.Columns().Len(); got != 0 {
		t.Fatalf("collector.Len = %d, want 0 after recovered panic; got %+v", got, collector.Columns().Findings())
	}
}

func TestCrossFilePhase_RecoversOnDemandModuleIndexRulePanics(t *testing.T) {
	panicRule := api.FakeRule("OnDemandModulePanic", api.WithNeeds(api.NeedsModuleIndex), api.WithCheck(func(ctx *api.Context) {
		ctx.Emit(scanner.Finding{File: "panic.kt", Line: 1, Message: "must be discarded"})
		panic("boom")
	}))
	graph := module.NewModuleGraph(t.TempDir())
	graph.Modules[":app"] = &module.Module{Path: ":app", Dir: t.TempDir()}
	in := DispatchResult{IndexResult: IndexResult{
		ParseResult: ParseResult{ActiveRules: []*api.Rule{panicRule}},
		Graph:       graph,
	}}
	result := CrossFileResult{}
	collector := scanner.NewFindingCollector(0)
	if err := (CrossFilePhase{}).runOnDemandModuleIndex(context.Background(), in, nil, collector, &result); err != nil {
		t.Fatalf("runOnDemandModuleIndex: %v", err)
	}
	assertPanicRule(t, result.Stats.Errors, "OnDemandModulePanic")
	if got := collector.Columns().Len(); got != 0 {
		t.Fatalf("collector.Len = %d, want 0 after recovered panic; got %+v", got, collector.Columns().Findings())
	}
}

func TestCrossFilePhaseKeepsDispatchPanicsSeparateFromCrossFilePanics(t *testing.T) {
	dispatchPanic := rules.DispatchError{RuleName: "DispatchPanic", FilePath: "z.kt", PanicValue: "dispatch boom"}
	in := DispatchResult{
		IndexResult: IndexResult{ParseResult: ParseResult{ActiveRules: []*api.Rule{
			api.FakeRule("CrossFilePanic", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(*api.Context) {
				panic("cross-file boom")
			})),
		}}},
		Stats: rules.RunStats{Errors: append(make([]rules.DispatchError, 0, 2), dispatchPanic)},
	}
	dispatchErrors := in.Stats.Errors
	wantDispatchErrors := slices.Clone(dispatchErrors)

	result, err := (CrossFilePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("CrossFilePhase.Run: %v", err)
	}
	if !slices.Equal(dispatchErrors, wantDispatchErrors) {
		t.Fatalf("dispatch errors = %+v, want unchanged %+v", dispatchErrors, wantDispatchErrors)
	}
	if len(result.Stats.Errors) != 2 || result.Stats.Errors[0].RuleName != "CrossFilePanic" {
		t.Fatalf("cross-file errors = %+v, want CrossFilePanic sorted before dispatch panic", result.Stats.Errors)
	}

	var warnings bytes.Buffer
	emitProjectAnalysisDiagnostics(&diag.Reporter{Warning: &warnings}, result.Stats.Errors, dispatchErrors, nil)
	got := warnings.String()
	if !strings.Contains(got, "krit: panic in rule CrossFilePanic") {
		t.Errorf("warnings = %q, want cross-file panic", got)
	}
	if strings.Contains(got, "krit: panic in rule DispatchPanic") {
		t.Errorf("warnings = %q, should not re-report dispatch panic", got)
	}
}

func TestAndroidGradleFindingsHonorInlineSuppression(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.gradle.kts")
	if err := os.WriteFile(path, []byte("// krit:ignore[GradleSuppressed]\nplugins {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := &api.Rule{
		ID: "GradleSuppressed", Category: "test", Description: "test", Scope: api.ScopeGradle, Needs: api.NeedsGradle,
		Check: func(ctx *api.Context) { ctx.EmitAt(1, 1, "must be suppressed") },
	}
	phase := AndroidPhase{}
	cols, _, _, _ := phase.runGradleOne(AndroidInput{Dispatcher: rules.NewDispatcher([]*api.Rule{rule}, nil)}, path)
	if cols.Len() != 0 {
		t.Fatalf("suppressed Gradle findings = %+v, want none", cols.Findings())
	}
}

func TestEmitProjectAnalysisDiagnosticsReportsProjectPanicsAndParseFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.gradle.kts")
	if err := os.WriteFile(path, []byte("plugins {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := &api.Rule{
		ID: "GradlePanic", Category: "test", Description: "test", Scope: api.ScopeGradle, Needs: api.NeedsGradle,
		Check: func(*api.Context) { panic("boom") },
	}
	result, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:     &android.Project{GradlePaths: []string{path}},
		ActiveRules: []*api.Rule{rule},
		Dispatcher:  rules.NewDispatcher([]*api.Rule{rule}, nil),
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	assertPanicRule(t, result.Stats.Errors, "GradlePanic")

	var warnings bytes.Buffer
	emitProjectAnalysisDiagnostics(
		&diag.Reporter{Warning: &warnings},
		result.Stats.Errors,
		nil,
		[]error{errors.New("parse failure")},
	)
	got := warnings.String()
	for _, want := range []string{"krit: panic in rule GradlePanic", "krit: 1 rule panic(s) during project analysis", "krit: 1 file(s) failed to parse"} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings = %q, want substring %q", got, want)
		}
	}
}

func assertPanicRule(t *testing.T, errs []rules.DispatchError, ruleID string) {
	t.Helper()
	if len(errs) != 1 || errs[0].RuleName != ruleID || errs[0].PanicValue != "boom" {
		t.Fatalf("Errors = %+v, want one recovered panic for %q", errs, ruleID)
	}
}
