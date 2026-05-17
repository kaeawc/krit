package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestRunProject_RoundTrip exercises the full new entry point:
// parse → index → dispatch → cross-file → output, against a tiny
// synthetic Kotlin project with one fake rule that flags class
// declarations. Asserts the output JSON contains the expected
// finding and the result counters reflect the input set.
func TestRunProject_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass X\n"), 0644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	rule := api.FakeRule("ProjectSmokeClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}

	if res.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1", res.FilesScanned)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1", res.FindingsCount)
	}
	if !strings.Contains(string(res.JSON), "ProjectSmokeClassDecl") {
		t.Errorf("output JSON does not contain rule name; got: %s", string(res.JSON))
	}

	// JSON output must be valid JSON. Decode-then-encode via a generic
	// map so we don't bind to a particular schema; it's a structural
	// smoke test.
	var probe map[string]any
	if err := json.Unmarshal(res.JSON, &probe); err != nil {
		t.Fatalf("Output JSON does not parse: %v\n--- payload ---\n%s", err, res.JSON)
	}
}

// TestRunProject_RejectsEmptyInputs guards the input contract: every
// caller-provided arg that's required must be present, and missing
// ones produce a clear error rather than nil-deref deeper in the
// pipeline.
func TestRunProject_RejectsEmptyInputs(t *testing.T) {
	cases := []struct {
		name string
		in   ProjectInput
		want string
	}{
		{
			name: "no config",
			in:   ProjectInput{Args: ProjectArgs{Paths: []string{"."}, ActiveRules: []*api.Rule{api.FakeRule("R")}}},
			want: "Config is required",
		},
		{
			name: "no rules",
			in:   ProjectInput{Args: ProjectArgs{Config: config.NewConfig(), Paths: []string{"."}}},
			want: "ActiveRules is empty",
		},
		{
			name: "no paths",
			in:   ProjectInput{Args: ProjectArgs{Config: config.NewConfig(), ActiveRules: []*api.Rule{api.FakeRule("R")}}},
			want: "Paths is empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunProject(context.Background(), tc.in)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// TestRunProject_DefaultsFormatToJSON confirms an empty Format field
// is treated as "json" (preserves the current `krit -f json` default
// without forcing every caller to spell it).
func TestRunProject_DefaultsFormatToJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "S.kt"),
		[]byte("package p\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{api.FakeRule("Noop")},
			// Format intentionally empty.
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(res.JSON, &probe); err != nil {
		t.Fatalf("Output is not JSON when Format empty: %v\n%s", err, res.JSON)
	}
}

func TestRunProjectAnalysis_RunsTargetedResolutionBeforeDispatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "S.kt")
	if err := os.WriteFile(path, []byte("package p\nfun f() { val x = 1 }\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resolver := &fakeResolver{}
	rule := &api.Rule{
		ID:            "ExprSelector",
		Category:      "test",
		Description:   "fake rule for testing",
		ExprPositions: func(*scanner.File) []uint32 { return []uint32{0} },
		Check:         func(*api.Context) {},
	}
	_, err := RunProjectAnalysis(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:             config.NewConfig(),
			Paths:              []string{dir},
			KotlinPaths:        []string{path},
			ActiveRules:        []*api.Rule{rule},
			TargetedResolution: true,
		},
		Host: ProjectHostState{
			TargetedExpressionResolver: resolver,
			TargetedExpressionSink:     newFakeSink(),
		},
	})
	if err != nil {
		t.Fatalf("RunProjectAnalysis: %v", err)
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("expected targeted resolver to run once; got %d", len(resolver.calls))
	}
	if len(resolver.calls[0][path]) == 0 {
		t.Fatalf("expected resolver request for %s; got %v", path, resolver.calls[0])
	}
}

// TestRunProjectAnalysis_ModuleAwareRule_RunsWithoutTracker is the
// regression test for the daemon warm-path bug where module-aware
// rules silently emitted zero findings when host.Tracker was nil
// (i.e. --perf not enabled).
//
// IndexPhase.runModuleIndexBuild used to early-return when
// in.ModuleParentTracker was nil, which left result.Graph and
// result.ModuleIndex unset. CrossFilePhase.collectModuleAwareFindings
// then bailed because in.Graph was nil and no NeedsModuleIndex rule
// ever ran. On Signal-Android with -all-rules this dropped ~31k
// findings (ModuleDeadCode, PackageDependencyCycle,
// VersionCatalogUnused, ...) from the daemon's response relative to
// in-process.
//
// The fix substitutes a no-op tracker when host.Tracker is nil so the
// IndexPhase steps still produce their outputs. This test pins that
// behavior by constructing a minimal multi-module Gradle project,
// registering a NeedsModuleIndex rule, and asserting it actually
// runs even when ProjectHostState carries no Tracker.
func TestRunProjectAnalysis_ModuleAwareRule_RunsWithoutTracker(t *testing.T) {
	dir := t.TempDir()
	// settings.gradle.kts with one explicit module so DiscoverModules
	// returns a non-empty Graph and ModuleHasAwareRule fires.
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"),
		[]byte("include(\":app\")\n"), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	appDir := filepath.Join(dir, "app")
	srcDir := filepath.Join(appDir, "src", "main", "kotlin")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "build.gradle.kts"),
		[]byte("plugins { kotlin(\"jvm\") }\n"), 0644); err != nil {
		t.Fatalf("write app build: %v", err)
	}
	ktPath := filepath.Join(srcDir, "Sample.kt")
	if err := os.WriteFile(ktPath,
		[]byte("package p\nclass X\n"), 0644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	// Register a fake NeedsModuleIndex rule that counts invocations.
	var moduleRuleCalls int
	rule := &api.Rule{
		ID:          "FakeModuleAwareRule",
		Category:    "test",
		Description: "module-aware test rule",
		Needs:       api.NeedsModuleIndex,
		Check: func(ctx *api.Context) {
			moduleRuleCalls++
			if ctx.ModuleIndex == nil {
				t.Errorf("module-aware rule got nil ModuleIndex")
			}
		},
	}

	// Crucial: leave host.Tracker unset (nil interface) — that's the
	// exact daemon-without-perf condition that triggered the bug.
	_, err := RunProjectAnalysis(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
		},
		Host: ProjectHostState{},
	})
	if err != nil {
		t.Fatalf("RunProjectAnalysis: %v", err)
	}
	if moduleRuleCalls == 0 {
		t.Fatalf("expected module-aware rule to run at least once with nil Tracker; got 0 calls")
	}
}
