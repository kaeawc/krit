package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Inline-snippet rule-test helpers. Pattern (borrowed from ktfmt's
// assertFormatted): one bug, one inline source snippet, one assertion.
// Use these for surgical regression tests of false positives or false
// negatives where a fixture would fragment the bug story across
// multiple files.
//
//   func TestRule_doesNotFlagX(t *testing.T) {
//       assertClean(t, "RuleName", `
//   package test
//   fun f() = ...
//   `)
//   }
//
// For broader behavioral coverage, prefer per-rule fixtures under
// tests/fixtures/. This file is for the cases where co-locating the
// snippet with the test is clearer than indirecting through a
// fixture file.

// inlineKotlin writes src to t.TempDir()/test.kt and parses it.
// (A Java variant can be added when the first Java caller appears.)
func inlineKotlin(t *testing.T, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write inline source: %v", err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("parse inline source: %v", err)
	}
	return f
}

// findRuleByID looks rule up in api.Registry. Failure is fatal.
func findRuleByID(t *testing.T, ruleID string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r != nil && r.ID == ruleID {
			return r
		}
	}
	t.Fatalf("rule %q not found in api.Registry", ruleID)
	return nil
}

// findingsFor runs rule against file in a single-rule dispatcher.
// The dispatcher only invokes the given rule, so cross-rule churn
// can't pollute the result.
func findingsFor(t *testing.T, rule *api.Rule, file *scanner.File) []scanner.Finding {
	t.Helper()
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	if rule.Needs.Has(api.NeedsResolver) {
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		d = rules.NewDispatcher([]*api.Rule{rule}, resolver)
	}
	cols := d.Run(file)
	return cols.Findings()
}

// flagsAssertion evaluates the assertFlagsOn predicate without
// touching testing.T. ok is true when the assertion would pass; msg
// is the failure message that would be reported when ok is false.
// Exposed as a separate function so unit tests can verify the
// predicate without driving a real test into a failure state.
func flagsAssertion(rule *api.Rule, file *scanner.File, findings []scanner.Finding) (ok bool, msg string) {
	if len(findings) > 0 {
		return true, ""
	}
	return false, fmt.Sprintf("%s: expected at least one finding on %s, got none", rule.ID, file.Path)
}

// cleanAssertion is the assertCleanOn predicate. See flagsAssertion.
func cleanAssertion(rule *api.Rule, file *scanner.File, findings []scanner.Finding) (ok bool, msg string) {
	if len(findings) == 0 {
		return true, ""
	}
	msgs := make([]string, 0, len(findings))
	for _, f := range findings {
		msgs = append(msgs, fmt.Sprintf("L%d: %s", f.Line, f.Message))
	}
	return false, fmt.Sprintf("%s: expected no findings on %s, got %d:\n  %s", rule.ID, file.Path, len(findings), strings.Join(msgs, "\n  "))
}

// assertFlagsOn fails the test if rule produces zero findings on
// file. Returns the findings for further inspection.
func assertFlagsOn(t *testing.T, rule *api.Rule, file *scanner.File) []scanner.Finding {
	t.Helper()
	findings := findingsFor(t, rule, file)
	if ok, msg := flagsAssertion(rule, file, findings); !ok {
		t.Fatal(msg)
	}
	return findings
}

// assertCleanOn fails the test if rule produces any findings on file.
func assertCleanOn(t *testing.T, rule *api.Rule, file *scanner.File) {
	t.Helper()
	findings := findingsFor(t, rule, file)
	if ok, msg := cleanAssertion(rule, file, findings); !ok {
		t.Fatal(msg)
	}
}

// assertFlags asserts that ruleID produces at least one finding when
// run over the inline Kotlin snippet src. Returns the findings.
func assertFlags(t *testing.T, ruleID, src string) []scanner.Finding {
	t.Helper()
	return assertFlagsOn(t, findRuleByID(t, ruleID), inlineKotlin(t, src))
}

// assertClean asserts that ruleID produces zero findings when run
// over the inline Kotlin snippet src.
func assertClean(t *testing.T, ruleID, src string) {
	t.Helper()
	assertCleanOn(t, findRuleByID(t, ruleID), inlineKotlin(t, src))
}
