package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Fixture layout for precompile rules:
//
//	tests/fixtures/precompile/positive/<RuleID>/<n>.kt   -> >=1 finding
//	tests/fixtures/precompile/negative/<RuleID>/<n>.kt   -> 0 findings
//	tests/fixtures/precompile/fixable/<RuleID>/<n>.kt    -> >=1 finding with Fix

const precompileFixtureRel = "tests/fixtures/precompile"

func precompileFixtureDir(t *testing.T) (string, bool) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(wd, "..", "..", precompileFixtureRel)
	if _, err := os.Stat(root); err != nil {
		return "", false
	}
	return root, true
}

func precompileRuleIndex() map[string]*api.Rule {
	idx := make(map[string]*api.Rule)
	for _, r := range api.Registry {
		if r.Category == api.CategoryPrecompile {
			idx[r.ID] = r
		}
	}
	return idx
}

func walkPrecompileFixtures(t *testing.T, kind string, fn func(t *testing.T, rule *api.Rule, file *scanner.File, findings []scanner.Finding)) {
	t.Helper()
	root, ok := precompileFixtureDir(t)
	if !ok {
		return
	}
	kindDir := filepath.Join(root, kind)
	if _, err := os.Stat(kindDir); err != nil {
		return
	}
	idx := precompileRuleIndex()

	entries, err := os.ReadDir(kindDir)
	if err != nil {
		t.Fatalf("reading %s: %v", kindDir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ruleID := entry.Name()
		rule, ok := idx[ruleID]
		if !ok {
			t.Run(kind+"/"+ruleID, func(t *testing.T) {
				t.Errorf("fixture directory %s/%s has no matching registered precompile rule", kind, ruleID)
			})
			continue
		}
		// Multi-file rules need a harness that this single-file
		// runner doesn't provide.
		if rule.Needs.Has(api.NeedsCrossFile) || rule.Needs.Has(api.NeedsModuleIndex) || rule.Needs.Has(api.NeedsParsedFiles) {
			t.Run(kind+"/"+ruleID, func(t *testing.T) {
				t.Skipf("rule %s requires multi-file context not yet wired", rule.ID)
			})
			continue
		}
		ruleDir := filepath.Join(kindDir, ruleID)
		fixtures, err := os.ReadDir(ruleDir)
		if err != nil {
			t.Errorf("reading %s: %v", ruleDir, err)
			continue
		}
		for _, fx := range fixtures {
			if fx.IsDir() || !strings.HasSuffix(fx.Name(), ".kt") {
				continue
			}
			path := filepath.Join(ruleDir, fx.Name())
			name := strings.TrimSuffix(fx.Name(), ".kt")
			t.Run(kind+"/"+ruleID+"/"+name, func(t *testing.T) {
				t.Parallel()
				file, err := scanner.ParseFile(context.Background(), path)
				if err != nil {
					t.Fatalf("parse %s: %v", path, err)
				}
				findings := runRule(t, rule, file)
				fn(t, rule, file, findings)
			})
		}
	}
}

func TestPrecompileFixturesPositive(t *testing.T) {
	walkPrecompileFixtures(t, "positive", func(t *testing.T, rule *api.Rule, _ *scanner.File, findings []scanner.Finding) {
		if len(findings) == 0 {
			t.Errorf("rule %s: expected >=1 finding for positive fixture, got 0", rule.ID)
		}
	})
}

func TestPrecompileFixturesNegative(t *testing.T) {
	walkPrecompileFixtures(t, "negative", func(t *testing.T, rule *api.Rule, _ *scanner.File, findings []scanner.Finding) {
		if len(findings) != 0 {
			t.Errorf("rule %s: expected 0 findings for negative fixture, got %d", rule.ID, len(findings))
			for _, f := range findings {
				t.Logf("  unexpected: %s:%d:%d %s", f.File, f.Line, f.Col, f.Message)
			}
		}
	})
}

func TestPrecompileFixturesFixable(t *testing.T) {
	walkPrecompileFixtures(t, "fixable", func(t *testing.T, rule *api.Rule, _ *scanner.File, findings []scanner.Finding) {
		if rule.Fix == api.FixNone {
			t.Errorf("rule %s: has fixable fixtures but Fix=FixNone in registration", rule.ID)
			return
		}
		if len(findings) == 0 {
			t.Errorf("rule %s: expected >=1 finding for fixable fixture, got 0", rule.ID)
			return
		}
		for _, f := range findings {
			if f.Fix != nil || f.BinaryFix != nil {
				return
			}
		}
		t.Errorf("rule %s: fixable fixture produced no Fix payload", rule.ID)
	})
}
