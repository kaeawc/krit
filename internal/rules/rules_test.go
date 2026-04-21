package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// fixtureRoot returns the absolute path to tests/fixtures relative to the repo root.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file location to find the repo root.
	// internal/rules/ -> repo root is ../../
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	root := filepath.Join(wd, "..", "..", "tests", "fixtures")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("fixture root not found at %s: %v", root, err)
	}
	return root
}

// buildRuleIndex creates a map from rule name to Rule for lookup.
func buildRuleIndex() map[string]*v2rules.Rule {
	idx := make(map[string]*v2rules.Rule, len(v2rules.Registry))
	for _, r := range v2rules.Registry {
		idx[r.ID] = r
	}
	return idx
}

// runRule runs a single rule against a parsed file using the dispatcher
// for correct single-pass behavior.
func runRule(t *testing.T, rule *v2rules.Rule, file *scanner.File) []scanner.Finding {
	t.Helper()
	if rule.Needs.Has(v2rules.NeedsResolver) {
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{rule}, resolver)
		cols := dispatcher.Run(file)
		return cols.Findings()
	}
	dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	cols := dispatcher.Run(file)
	return cols.Findings()
}

func TestPositiveFixtures(t *testing.T) {
	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive")
	ruleIndex := buildRuleIndex()

	count := 0
	err := filepath.Walk(positiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}

		// Extract rule name from filename (e.g., MagicNumber.kt -> MagicNumber)
		ruleName := strings.TrimSuffix(info.Name(), ".kt")

		rule, ok := ruleIndex[ruleName]
		if !ok {
			t.Logf("SKIP: no rule registered for fixture %s", ruleName)
			return nil
		}

		// Skip project-scope rules that need more than one parsed file.
		if rule.Needs.Has(v2rules.NeedsParsedFiles) {
			return nil
		}
		if rule.Needs.Has(v2rules.NeedsCrossFile) {
			return nil
		}
		if rule.Needs.Has(v2rules.NeedsModuleIndex) {
			return nil
		}

		count++
		t.Run("positive/"+ruleName, func(t *testing.T) {
			t.Parallel()
			file, err := scanner.ParseFile(path)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", path, err)
			}

			findings := runRule(t, rule, file)
			if len(findings) == 0 {
				t.Errorf("expected findings > 0 for positive fixture %s, got 0", ruleName)
			}
		})

		return nil
	})
	if err != nil {
		t.Fatalf("walking positive fixtures: %v", err)
	}
	if count == 0 {
		t.Fatal("no positive fixtures found")
	}
	t.Logf("ran %d positive fixture tests", count)
}

func TestNegativeFixtures(t *testing.T) {
	root := fixtureRoot(t)
	negativeDir := filepath.Join(root, "negative")
	ruleIndex := buildRuleIndex()

	count := 0
	err := filepath.Walk(negativeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}

		ruleName := strings.TrimSuffix(info.Name(), ".kt")

		rule, ok := ruleIndex[ruleName]
		if !ok {
			t.Logf("SKIP: no rule registered for fixture %s", ruleName)
			return nil
		}

		if rule.Needs.Has(v2rules.NeedsParsedFiles) {
			return nil
		}
		if rule.Needs.Has(v2rules.NeedsCrossFile) {
			return nil
		}

		count++
		t.Run("negative/"+ruleName, func(t *testing.T) {
			t.Parallel()
			file, err := scanner.ParseFile(path)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", path, err)
			}

			findings := runRule(t, rule, file)
			if len(findings) != 0 {
				t.Errorf("expected 0 findings for negative fixture %s, got %d:", ruleName, len(findings))
				for _, f := range findings {
					t.Logf("  %s:%d:%d %s", f.File, f.Line, f.Col, f.Message)
				}
			}
		})

		return nil
	})
	if err != nil {
		t.Fatalf("walking negative fixtures: %v", err)
	}
	if count == 0 {
		t.Fatal("no negative fixtures found")
	}
	t.Logf("ran %d negative fixture tests", count)
}

func TestDescriptionOfReturnsDescription(t *testing.T) {
	// Every rule in the registry should have a non-empty Description
	// since Description is a field on v2.Rule.
	for _, r := range v2rules.Registry {
		if r.Description == "" {
			t.Errorf("rule %q has empty Description", r.ID)
		}
	}
}

func TestDescriptionOfReturnsNonEmptyForProviders(t *testing.T) {
	idx := buildRuleIndex()
	for _, name := range []string{"LongMethod", "CyclomaticComplexMethod", "GlobalCoroutineUsage"} {
		r, ok := idx[name]
		if !ok {
			t.Errorf("rule %q not in registry", name)
			continue
		}
		if r.Description == "" {
			t.Errorf("rule %q has empty Description", name)
		}
	}
}
