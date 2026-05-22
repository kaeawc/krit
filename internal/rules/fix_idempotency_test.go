package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// knownNonIdempotentFixableRules names rules whose canonical fixed
// output (the .expected file) still produces fixable findings when the
// same rule is re-run. Each entry should have a short reason so the
// allowlist doesn't rot. Empty list = strict idempotency for every
// fixable rule.
//
// To remove an entry: either fix the rule so its first-pass output is
// a fixed point of the rule, OR (if applying the fix to convergence
// is the intended behavior) re-run the fixable-fixtures test with
// UPDATE_FIXABLE_EXPECTED=1 after iterating fixes to a fixed point.
var knownNonIdempotentFixableRules = map[string]string{}

// TestFixableFixturesIdempotent re-runs fixable rules over their
// .expected files and asserts that no fixable findings remain. This
// catches fixes that produce output the same rule re-flags — i.e., a
// fix that is not a fixed point of its rule.
//
// Why this matters: ktfmt's correctness story is "format(format(x))
// == format(x)". The same property applies to autofixes — applying a
// fix twice should be a no-op. If a fix produces output the rule
// re-flags, users see warnings on already-fixed code or get caught in
// a fix pingpong.
//
// We use the canonical .expected file as input. If .expected drifts
// from actual fix output, TestFixableFixtures catches that
// independently.
func TestFixableFixturesIdempotent(t *testing.T) {
	root := fixtureRoot(t)
	fixableDir := filepath.Join(root, "fixable")

	var activeRules []*api.Rule
	for _, r := range api.Registry {
		if rules.IsDefaultActive(r.ID) {
			activeRules = append(activeRules, r)
		}
	}

	bundled, err := runBundledIdempotencyChecks(t, fixableDir, activeRules)
	if err != nil {
		t.Fatalf("bundled idempotency: %v", err)
	}
	perRule, err := runPerRuleIdempotencyChecks(t, filepath.Join(fixableDir, "per-rule"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("per-rule idempotency: %v", err)
	}
	total := bundled + perRule
	if total == 0 {
		t.Fatal("no fixable .expected files found for idempotency check")
	}
	t.Logf("ran %d idempotency checks (%d bundled, %d per-rule)", total, bundled, perRule)
}

func runBundledIdempotencyChecks(t *testing.T, dir string, activeRules []*api.Rule) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !isFixableSourceFixture(entry.Name()) {
			continue
		}
		expectedPath := filepath.Join(dir, entry.Name()+".expected")
		if _, err := os.Stat(expectedPath); err != nil {
			continue
		}
		name := fixableFixtureRuleName(entry.Name())
		count++
		t.Run("idempotent/bundled/"+name, func(t *testing.T) {
			leftovers, err := findFixableFindings(expectedPath, activeRules, "")
			if err != nil {
				t.Fatalf("running rules over %s: %v", expectedPath, err)
			}
			if len(leftovers) == 0 {
				return
			}
			reportIdempotencyViolation(t, name, expectedPath, leftovers, true)
		})
	}
	return count, nil
}

func runPerRuleIdempotencyChecks(t *testing.T, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	byName := make(map[string]*api.Rule, len(api.Registry))
	for _, r := range api.Registry {
		byName[r.ID] = r
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !isFixableSourceFixture(entry.Name()) {
			continue
		}
		expectedPath := filepath.Join(dir, entry.Name()+".expected")
		if _, err := os.Stat(expectedPath); err != nil {
			continue
		}
		name := fixableFixtureRuleName(entry.Name())
		rule, ok := byName[name]
		if !ok {
			// Mismatched name is reported by TestFixableFixtures; skip
			// here so we don't double-report.
			continue
		}
		count++
		t.Run("idempotent/per-rule/"+name, func(t *testing.T) {
			leftovers, err := findFixableFindings(expectedPath, []*api.Rule{rule}, name)
			if err != nil {
				t.Fatalf("running rule %s over %s: %v", name, expectedPath, err)
			}
			if len(leftovers) == 0 {
				return
			}
			reportIdempotencyViolation(t, name, expectedPath, leftovers, false)
		})
	}
	return count, nil
}

// findFixableFindings parses sourcePath and runs ruleSet against it,
// returning every finding whose Fix is non-nil. If ruleFilter is
// non-empty, only findings whose Rule matches are returned (used for
// per-rule fixtures so cross-rule output churn doesn't masquerade as
// non-idempotency of the rule under test).
func findFixableFindings(sourcePath string, ruleSet []*api.Rule, ruleFilter string) ([]scanner.Finding, error) {
	file, err := parseFixableSourceFixture(sourcePath)
	if err != nil {
		return nil, err
	}
	dispatcher := rules.NewDispatcher(ruleSet, nil)
	if rulesNeedResolver(ruleSet) {
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		dispatcher = rules.NewDispatcher(ruleSet, resolver)
	}
	cols := dispatcher.Run(file)
	findings := cols.Findings()
	var leftovers []scanner.Finding
	for _, f := range findings {
		if f.Fix == nil {
			continue
		}
		if ruleFilter != "" && f.Rule != ruleFilter {
			continue
		}
		leftovers = append(leftovers, f)
	}
	return leftovers, nil
}

// reportIdempotencyViolation fails the test (or logs, if allowlisted)
// for a fixture whose .expected output still has fixable findings.
// The bundled flag distinguishes bundled fixtures (where the
// allowlist applies if every offending rule is allowlisted) from
// per-rule fixtures (allowlist keyed by the single rule under test).
func reportIdempotencyViolation(t *testing.T, name, expectedPath string, leftovers []scanner.Finding, bundled bool) {
	t.Helper()

	byRule := make(map[string]int, len(leftovers))
	for _, f := range leftovers {
		byRule[f.Rule]++
	}

	// Per-rule: allowlist keyed by rule name. Bundled: allowlist any
	// rule whose findings appear (the bundled fixture is a single
	// canonical converged file, so any allowlisted rule mutes it).
	if !bundled {
		if reason, ok := knownNonIdempotentFixableRules[name]; ok {
			t.Logf("%s: %d fixable finding(s) on already-fixed output (allowlisted: %s)", name, len(leftovers), reason)
			return
		}
	} else {
		allAllowlisted := len(byRule) > 0
		var matchedReasons []string
		for r := range byRule {
			reason, ok := knownNonIdempotentFixableRules[r]
			if !ok {
				allAllowlisted = false
				break
			}
			matchedReasons = append(matchedReasons, fmt.Sprintf("%s: %s", r, reason))
		}
		if allAllowlisted {
			sort.Strings(matchedReasons)
			t.Logf("%s: %d fixable finding(s) on already-fixed output (allowlisted: %s)", name, len(leftovers), strings.Join(matchedReasons, "; "))
			return
		}
	}

	summaries := make([]string, 0, len(byRule))
	for r, n := range byRule {
		summaries = append(summaries, fmt.Sprintf("%s (%d)", r, n))
	}
	sort.Strings(summaries)

	t.Errorf(`%s: fix is not idempotent — running rules over %s produced %d fixable finding(s): %s

The .expected file represents the fixed output, but a rule still flags it as fixable.
Either:
  1. Fix the rule so its first-pass output is a fixed point (preferred).
  2. If multiple passes are intended, regenerate the .expected by running fixes to convergence and update TestFixableFixtures (UPDATE_FIXABLE_EXPECTED=1).
  3. Add the rule to knownNonIdempotentFixableRules with a reason and a follow-up issue link.`,
		name, expectedPath, len(leftovers), strings.Join(summaries, ", "))
}
