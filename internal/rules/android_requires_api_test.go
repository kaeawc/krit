package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// findRequiresAPIViolationRule looks up the registered RequiresApiViolation
// descriptor so tests can run it through the same code path as production.
func findRequiresAPIViolationRule(t *testing.T) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "RequiresApiViolation" {
			return r
		}
	}
	t.Fatalf("RequiresApiViolation not registered")
	return nil
}

// runRequiresApi mirrors the dispatcher entry point used by other oracle-aware
// rule tests so that NeedsOracleCallTargets-decorated rules see a real
// resolver-less invocation path. The rule falls back to source-level
// annotation scanning when the oracle is absent, which is the path exercised
// here.
func runRequiresApi(t *testing.T, source string, minSdk int) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	rule := findRequiresAPIViolationRule(t)
	dispatcher := NewDispatcher([]*api.Rule{rule}, nil)
	dispatcher.SetLibraryFacts(librarymodel.FactsForProfile(librarymodel.ProjectProfile{
		MinSdkVersion: minSdk,
	}))
	cols := dispatcher.Run(file)
	return cols.Findings()
}

func TestRequiresApiViolation_PositionalArgument(t *testing.T) {
	source := `package test
import androidx.annotation.RequiresApi
@RequiresApi(26)
fun helper() {}
fun caller() { helper() }
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "requires API 26") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "minSdk is 21") {
		t.Errorf("expected minSdk 21 in message: %q", findings[0].Message)
	}
}

func TestRequiresApiViolation_NamedValueArgument(t *testing.T) {
	source := `package test
import androidx.annotation.RequiresApi
@RequiresApi(value = 30)
fun helper() {}
fun caller() { helper() }
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "requires API 30") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestRequiresApiViolation_TargetApiAlsoFlagged(t *testing.T) {
	source := `package test
import android.annotation.TargetApi
@TargetApi(26)
fun helper() {}
fun caller() { helper() }
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for @TargetApi, got %d", len(findings))
	}
}

func TestRequiresApiViolation_SkipsWhenMinSdkAtOrAboveRequirement(t *testing.T) {
	source := `package test
import androidx.annotation.RequiresApi
@RequiresApi(26)
fun helper() {}
fun caller() { helper() }
`
	findings := runRequiresApi(t, source, 26)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when minSdk meets requirement, got %d", len(findings))
	}
}

func TestRequiresApiViolation_SdkIntGuardSuppresses(t *testing.T) {
	source := `package test
import android.os.Build
import androidx.annotation.RequiresApi
@RequiresApi(26)
fun helper() {}
fun caller() {
    if (Build.VERSION.SDK_INT >= 26) {
        helper()
    }
}
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings under SDK_INT guard, got %d: %+v", len(findings), findings)
	}
}

func TestRequiresApiViolation_RequiresApiAnnotationOnCallerSuppresses(t *testing.T) {
	source := `package test
import androidx.annotation.RequiresApi
@RequiresApi(26)
fun helper() {}
class Caller {
    @RequiresApi(26)
    fun guarded() { helper() }
}
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when caller annotated, got %d", len(findings))
	}
}

func TestRequiresApiViolation_InheritsFromEnclosingClassAnnotation(t *testing.T) {
	source := `package test
import androidx.annotation.RequiresApi
@RequiresApi(26)
class NewApi {
    fun helper() {}
}
fun caller(api: NewApi) { api.helper() }
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from inherited class @RequiresApi, got %d", len(findings))
	}
}

func TestRequiresApiViolation_UnrelatedCallsAreSilent(t *testing.T) {
	source := `package test
fun helper() {}
fun caller() { helper() }
`
	findings := runRequiresApi(t, source, 21)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings without @RequiresApi, got %d", len(findings))
	}
}

func TestRequiresApiViolation_ParseRequiresApiAnnotationLevel(t *testing.T) {
	cases := map[string]int{
		"26":             26,
		"value = 30":     30,
		"api = 31":       31,
		" 21 , extra ":   21,
		"":               0,
		"value = \"26\"": 0, // non-numeric must not parse
	}
	for input, want := range cases {
		if got := parseRequiresAPIArgText(input); got != want {
			t.Errorf("parseRequiresAPIArgText(%q) = %d, want %d", input, got, want)
		}
	}
}
