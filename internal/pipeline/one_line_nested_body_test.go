package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// runPrintlnProject runs PrintlnInProduction over one Kotlin file through the
// whole project pipeline, including the parse-error-region output filter, and
// returns the reported lines plus the number of findings that filter dropped.
func runPrintlnProject(t *testing.T, src string) ([]string, int) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "H.kt"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := RunProject(context.Background(), ProjectInput{Args: ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{root},
		ActiveRules: []*api.Rule{findV2RuleForTest(t, "PrintlnInProduction")},
		Format:      "json",
		Version:     "test",
	}})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	var got []string
	for _, f := range res.FinalFindings.Findings() {
		got = append(got, fmt.Sprintf("%d:%d", f.Line, f.Col))
	}
	sort.Strings(got)
	return got, res.Stats.FindingsInErrorRegions
}

// A one-line nested class body used to turn the whole file into a parse-error
// region, so every finding in the file was dropped before output (#1839).
func TestRunProject_OneLineNestedBodyKeepsFileFindings(t *testing.T) {
	tests := []struct {
		name, line string
	}{
		{"companion object", "class H { companion object { val X = 1 } }"},
		{"object", "class H { object O { const val X = 1 } }"},
		{"nested class", "class H { class I { val x = 1 } }"},
		{"nested interface", "class H { interface I { fun f() } }"},
		{"enum body", "class H { enum class E { A, B; fun g() = 1 } }"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.line + "\n\nfun f() { println(1) }\n\nval magic = 42\n"
			got, dropped := runPrintlnProject(t, src)
			if want := []string{"3:11"}; strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("findings = %v, want %v", got, want)
			}
			if dropped != 0 {
				t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
			}
		})
	}

	t.Run("finding inside the nested body", func(t *testing.T) {
		got, dropped := runPrintlnProject(t, "class H { companion object { fun g() = println(2) } }\n")
		if want := []string{"1:40"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("findings = %v, want %v", got, want)
		}
		if dropped != 0 {
			t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
		}
	})
}

// @Suppress on a one-line class still covers only that class: the println
// inside its nested body is suppressed and the top-level one is reported.
func TestRunProject_SuppressOnOneLineNestedBodyStaysScoped(t *testing.T) {
	src := "@Suppress(\"PrintlnInProduction\") class H { companion object { fun g() = println(2) } }\n\nfun f() { println(1) }\n"
	got, dropped := runPrintlnProject(t, src)
	if want := []string{"3:11"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("findings = %v, want %v", got, want)
	}
	if dropped != 0 {
		t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
	}
}
