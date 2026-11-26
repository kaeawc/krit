package rules

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Lint reports production calls to @VisibleForTesting members as VisibleForTests; krit's equivalent is
// VisibleForTestingCallerInNonTest, so the Lint ID must suppress it (and carry lint.xml and baseline entries over).
func TestVisibleForTestsIsALintAliasOfVisibleForTestingCallerInNonTest(t *testing.T) {
	if got := AllSuppressionAliases()["VisibleForTestingCallerInNonTest"]; !slices.Contains(got, "VisibleForTests") {
		t.Fatalf("aliases of VisibleForTestingCallerInNonTest = %v, want VisibleForTests among them", got)
	}
	path := filepath.Join(t.TempDir(), "Main.kt")
	src := `package p

import android.annotation.SuppressLint

@SuppressLint("VisibleForTests")
fun caller() = target()
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	filter := scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases())
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "VisibleForTestingCallerInNonTest" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("VisibleForTestingCallerInNonTest is not registered")
	}
	if !filter.IsSuppressedAtColumn(rule.ID, rule.Category, 6, 16) {
		t.Fatal(`@SuppressLint("VisibleForTests") does not suppress VisibleForTestingCallerInNonTest`)
	}
}
