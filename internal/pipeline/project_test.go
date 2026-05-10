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
