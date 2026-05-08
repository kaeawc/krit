package scan

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestCompletionsFilename(t *testing.T) {
	cases := []struct {
		shell    string
		wantPath string
		wantOK   bool
	}{
		{"bash", "completions/krit.bash", true},
		{"zsh", "completions/krit.zsh", true},
		{"fish", "completions/krit.fish", true},
		{"", "", false},
		{"powershell", "", false},
		{"BASH", "", false},
		{"sh", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			gotPath, gotOK := completionsFilename(tc.shell)
			if gotPath != tc.wantPath || gotOK != tc.wantOK {
				t.Fatalf("completionsFilename(%q) = (%q, %v); want (%q, %v)",
					tc.shell, gotPath, gotOK, tc.wantPath, tc.wantOK)
			}
		})
	}
}

func TestCompletionsFSResolvesAllShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		filename, ok := completionsFilename(shell)
		if !ok {
			t.Fatalf("completionsFilename(%q) returned ok=false", shell)
		}
		data, err := completionsFS.ReadFile(filename)
		if err != nil {
			t.Fatalf("completionsFS.ReadFile(%q) for shell %q: %v", filename, shell, err)
		}
		if len(data) == 0 {
			t.Fatalf("completion script for %q is empty", shell)
		}
	}
}

func TestInitStarterConfigContents(t *testing.T) {
	if initStarterConfig == "" {
		t.Fatal("initStarterConfig is empty")
	}
	wantSubstrings := []string{
		"MagicNumber:",
		"MaxLineLength:",
		"ReturnCount:",
		"LongMethod:",
		"CyclomaticComplexMethod:",
		"FunctionNaming:",
		"UnsafeCast:",
	}
	for _, want := range wantSubstrings {
		if !contains(initStarterConfig, want) {
			t.Fatalf("initStarterConfig missing %q", want)
		}
	}
}

func TestComputeListRulesSummaryEmpty(t *testing.T) {
	got := computeListRulesSummary(nil)
	if got != (listRulesSummary{}) {
		t.Fatalf("nil registry: got %+v, want zero value", got)
	}
}

func TestComputeListRulesSummaryRealRegistry(t *testing.T) {
	got := computeListRulesSummary(api.Registry)
	if got.Total != len(api.Registry) {
		t.Fatalf("Total = %d, want %d", got.Total, len(api.Registry))
	}
	if got.Active < 0 || got.Active > got.Total {
		t.Fatalf("Active = %d, want 0..%d", got.Active, got.Total)
	}
	if got.Fixable < 0 || got.Fixable > got.Total {
		t.Fatalf("Fixable = %d, want 0..%d", got.Fixable, got.Total)
	}
	if got.Total == 0 {
		t.Fatal("expected the live api.Registry to have at least one rule")
	}
}

func TestRunOutputTypesFlagNoOpWhenEmpty(t *testing.T) {
	// Empty OutputPath must short-circuit before any oracle / filesystem
	// work. If this ever calls os.Exit, the test process dies and `go test`
	// reports the panic — which is the assertion.
	runOutputTypesFlag(outputTypesOpts{OutputPath: ""})
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
