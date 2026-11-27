package rules_test

import (
	"slices"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func inclusiveRule(t *testing.T) *rules.InclusiveNamingRule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "InclusiveNaming" {
			return r.Implementation.(*rules.InclusiveNamingRule)
		}
	}
	t.Fatal("InclusiveNaming not registered")
	return nil
}

// With no block list, the default, the rule reports nothing; the list's entries are split on commas, and one with
// upper-case letters never matches the lower-cased name, as in Lint.
func TestInclusiveNamingBlockList(t *testing.T) {
	const src = "package app\n\nclass MasterView\n\nfun whitelistAll() = Unit\n"
	r := inclusiveRule(t)
	t.Cleanup(func() { r.BlockList = nil })
	for _, tc := range []struct {
		name  string
		list  []string
		lines []int
	}{
		{"empty", nil, nil},
		{"comma-separated entry", []string{"master, whitelist"}, []int{3, 5}},
		{"separate entries", []string{"whitelist", "master"}, []int{3, 5}},
		{"upper-case entry", []string{"Master"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r.BlockList = tc.list
			var lines []int
			for _, f := range runRuleByName(t, "InclusiveNaming", src) {
				lines = append(lines, f.Line)
			}
			if !slices.Equal(lines, tc.lines) {
				t.Errorf("want findings on lines %v, got %v", tc.lines, lines)
			}
		})
	}
}

// slack-lints' DeprecatedCall is a Lint alias of Deprecation, so a suppression written for it applies.
func TestDeprecatedCallAliasesDeprecation(t *testing.T) {
	for _, r := range api.Registry {
		if r.ID == "Deprecation" {
			if aliases := rules.LintSuppressionAliases(r); !slices.Contains(aliases, "DeprecatedCall") {
				t.Errorf("Deprecation Lint aliases = %v, want DeprecatedCall", aliases)
			}
			return
		}
	}
	t.Fatal("Deprecation not registered")
}
