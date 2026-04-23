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

// fixtureNames collects rule fixture coverage under dir. It recognises two
// fixture forms:
//
//  1. A flat .kt file:  <dir>/<category>/<RuleName>.kt
//  2. A sub-directory:  <dir>/<category>/<any-name>/   (used for Gradle rules
//     whose fixture files are .gradle.kts, not .kt)
//
// Sub-directory names are normalised (lower-case, non-alphanumeric stripped)
// and matched case-insensitively against each rule ID so that, e.g.,
// "all-projects-block" maps to "AllProjectsBlock".
func fixtureNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := make(map[string]bool)

	// Normalise a string to lowercase alphanumeric only for fuzzy matching.
	norm := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

	// First pass: collect flat .kt fixtures (exact rule-name match) and all
	// sub-directory normalised names.
	subdirNorms := make(map[string]bool)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		depth := len(strings.Split(rel, string(os.PathSeparator)))
		if info.IsDir() {
			if depth == 2 { // <category>/<subdir> — Gradle-style fixture dir
				subdirNorms[norm(info.Name())] = true
			}
			return nil
		}
		if strings.HasSuffix(path, ".kt") && depth == 2 { // <category>/<RuleName>.kt
			out[strings.TrimSuffix(info.Name(), ".kt")] = true
		}
		return nil
	})

	// Second pass: resolve sub-directory fixtures against registered rule IDs.
	for _, r := range v2rules.Registry {
		if subdirNorms[norm(r.ID)] {
			out[r.ID] = true
		}
	}
	return out
}

// TestNonAndroidRulesHaveFixtures enforces that every non-Android rule that
// can run against a single file has both a positive and a negative fixture.
// Rules requiring NeedsCrossFile, NeedsModuleIndex, or NeedsParsedFiles are
// excluded because the existing TestPositiveFixtures/TestNegativeFixtures
// already skip them. Rules in the "android-lint" category (AndroidDeps != 0
// or Category == "android-lint") are tracked separately in the android-lint
// cluster.
func TestNonAndroidRulesHaveFixtures(t *testing.T) {
	root := fixtureRoot(t)
	positiveFixtures := fixtureNames(t, filepath.Join(root, "positive"))
	negativeFixtures := fixtureNames(t, filepath.Join(root, "negative"))

	var missingPositive, missingNegative []string
	for _, r := range v2rules.Registry {
		// Skip Android-specific rules (by explicit deps or category).
		if r.AndroidDeps != 0 || r.Category == "android-lint" {
			continue
		}
		// Skip rules that require multi-file analysis.
		if r.Needs.Has(v2rules.NeedsCrossFile) || r.Needs.Has(v2rules.NeedsModuleIndex) || r.Needs.Has(v2rules.NeedsParsedFiles) {
			continue
		}
		if !positiveFixtures[r.ID] {
			missingPositive = append(missingPositive, r.ID)
		}
		if !negativeFixtures[r.ID] {
			missingNegative = append(missingNegative, r.ID)
		}
	}

	if len(missingPositive) > 0 {
		t.Errorf("rules missing positive fixture (%d) — add tests/fixtures/positive/<category>/<RuleName>.kt:\n  %s",
			len(missingPositive), strings.Join(missingPositive, "\n  "))
	}
	if len(missingNegative) > 0 {
		t.Errorf("rules missing negative fixture (%d) — add tests/fixtures/negative/<category>/<RuleName>.kt:\n  %s",
			len(missingNegative), strings.Join(missingNegative, "\n  "))
	}
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
