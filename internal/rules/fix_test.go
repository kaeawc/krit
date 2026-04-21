package rules_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestFixableFixtures(t *testing.T) {
	root := fixtureRoot(t)
	fixableDir := filepath.Join(root, "fixable")

	// Use only default-active rules (matches CLI behavior)
	var activeRules []*v2rules.Rule
	for _, r := range v2rules.Registry {
		if rules.IsDefaultActive(r.ID) {
			activeRules = append(activeRules, r)
		}
	}
	dispatcher := rules.NewDispatcherV2(activeRules)

	// Bundled fixtures live directly under tests/fixtures/fixable/
	// and run every default-active rule, applying every fixable
	// finding. Filename must match a corresponding .expected file.
	bundledCount, err := runBundledFixableFixtures(t, fixableDir, dispatcher)
	if err != nil {
		t.Fatalf("bundled fixtures: %v", err)
	}

	// Per-rule fixtures live under tests/fixtures/fixable/per-rule/
	// and run ONLY the named rule. The filename (without .kt) must
	// match a registered rule's Name() exactly. This is the
	// roadmap/17 Phase 4 target — every fixable rule gets its own
	// isolated fixture covering a canonical fix scenario.
	perRuleDir := filepath.Join(fixableDir, "per-rule")
	perRuleCount, err := runPerRuleFixableFixtures(t, perRuleDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("per-rule fixtures: %v", err)
	}

	total := bundledCount + perRuleCount
	if total == 0 {
		t.Fatal("no fixable fixtures found")
	}
	t.Logf("ran %d fixable fixture tests (%d bundled, %d per-rule)", total, bundledCount, perRuleCount)
}

// runBundledFixableFixtures walks the top-level fixable/ directory and
// runs each <Name>.kt / <Name>.kt.expected pair against every active
// rule (the original pre-Phase-4 behavior).
func runBundledFixableFixtures(t *testing.T, dir string, dispatcher *rules.Dispatcher) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kt") {
			continue
		}
		expectedPath := filepath.Join(dir, entry.Name()+".expected")
		if _, err := os.Stat(expectedPath); err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".kt")
		ktPath := filepath.Join(dir, entry.Name())

		count++
		t.Run("fixable/"+name, func(t *testing.T) {
			file, err := scanner.ParseFile(ktPath)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", ktPath, err)
			}

			findingCols := dispatcher.Run(file)
			findings := findingCols.Findings()
			var fixableFindings []scanner.Finding
			for _, f := range findings {
				if f.Fix != nil {
					fixableFindings = append(fixableFindings, f)
				}
			}
			if len(fixableFindings) == 0 {
				t.Skipf("no fixable findings for %s", name)
				return
			}

			applyAndCompare(t, ktPath, entry.Name(), fixableFindings, expectedPath, name)
		})
	}
	return count, nil
}

// runPerRuleFixableFixtures walks the fixable/per-rule/ directory. Each
// <RuleName>.kt / <RuleName>.kt.expected pair is run through ONLY the
// rule whose Name() matches <RuleName>. This gives per-rule coverage
// that isolates the fix under test from any other rule's output on
// the same input.
func runPerRuleFixableFixtures(t *testing.T, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	// Index rules by name for O(1) lookup by fixture filename.
	byName := make(map[string]*v2rules.Rule, len(v2rules.Registry))
	for _, r := range v2rules.Registry {
		byName[r.ID] = r
	}

	bootstrap := os.Getenv("UPDATE_FIXABLE_EXPECTED") == "1"

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kt") {
			continue
		}
		expectedPath := filepath.Join(dir, entry.Name()+".expected")
		if _, err := os.Stat(expectedPath); err != nil {
			if !bootstrap {
				// Fixture without an expected — skip silently unless
				// we're explicitly regenerating. This lets authors
				// drop in new <RuleName>.kt files and run the test
				// with UPDATE_FIXABLE_EXPECTED=1 to bootstrap the
				// matching .expected file.
				continue
			}
		}

		name := strings.TrimSuffix(entry.Name(), ".kt")
		rule, ok := byName[name]
		if !ok {
			t.Run("fixable/per-rule/"+name, func(t *testing.T) {
				t.Fatalf("per-rule fixture %s.kt does not match any registered rule", name)
			})
			continue
		}
		ktPath := filepath.Join(dir, entry.Name())

		count++
		t.Run("fixable/per-rule/"+name, func(t *testing.T) {
			file, err := scanner.ParseFile(ktPath)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", ktPath, err)
			}

			// Run the single rule in isolation. Every fixable rule is
			// either a flat-dispatch rule, line rule, aggregate rule,
			// cross-file rule, module-aware rule, or legacy Rule; the
			// dispatcher handles all of those when given a singleton
			// registry.
			soloDispatcher := rules.NewDispatcherV2([]*v2rules.Rule{rule})
			if rule.Needs.Has(v2rules.NeedsResolver) {
				resolver := typeinfer.NewResolver()
				resolver.IndexFilesParallel([]*scanner.File{file}, 1)
				soloDispatcher = rules.NewDispatcherV2([]*v2rules.Rule{rule}, resolver)
			}
			findingCols := soloDispatcher.Run(file)
			findings := findingCols.Findings()

			var fixableFindings []scanner.Finding
			for _, f := range findings {
				if f.Rule != name {
					continue
				}
				if f.Fix != nil {
					fixableFindings = append(fixableFindings, f)
				}
			}
			if len(fixableFindings) == 0 {
				if bootstrap {
					t.Skipf("bootstrap: rule %s produced no fixable findings for per-rule/%s.kt; remove or rewrite the fixture", name, name)
					return
				}
				t.Fatalf("per-rule fixture %s.kt triggers %d findings but none have Fix populated — rule isn't fixing this input", name, len(findings))
			}

			applyAndCompare(t, ktPath, entry.Name(), fixableFindings, expectedPath, name)
		})
	}
	return count, nil
}

// applyAndCompare applies the given findings' fixes to a temporary
// copy of ktPath and diffs the result against expectedPath. Set
// UPDATE_FIXABLE_EXPECTED=1 to regenerate the .expected file from
// the applied output instead of comparing.
func applyAndCompare(t *testing.T, ktPath, baseName string, findings []scanner.Finding, expectedPath, name string) {
	t.Helper()
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, baseName)
	srcContent, _ := os.ReadFile(ktPath)
	if err := os.WriteFile(tmpPath, srcContent, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	for i := range findings {
		findings[i].File = tmpPath
	}

	columns := scanner.CollectFindings(findings)
	nFixes, _, fixErrs := fixer.ApplyAllFixesColumns(&columns, "")
	if len(fixErrs) > 0 {
		t.Fatalf("ApplyAllFixesColumns error: %v", fixErrs[0])
	}
	if nFixes == 0 {
		t.Fatalf("no fixes applied for %s (had %d fixable findings)", name, len(findings))
	}

	gotBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("reading fixed file: %v", err)
	}

	if os.Getenv("UPDATE_FIXABLE_EXPECTED") == "1" {
		if err := os.WriteFile(expectedPath, gotBytes, 0644); err != nil {
			t.Fatalf("regenerating %s: %v", expectedPath, err)
		}
		t.Logf("regenerated %s", expectedPath)
		return
	}

	expectedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("reading expected file: %v", err)
	}

	got := string(gotBytes)
	expected := string(expectedBytes)
	if got != expected {
		t.Errorf("fixed output for %s does not match expected.\n--- got ---\n%s\n--- expected ---\n%s",
			name, truncate(got, 2000), truncate(expected, 2000))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}

// TestFixableRulesHavePerRuleFixture asserts that every rule
// advertised as fixable via the FixableRule interface has a
// corresponding per-rule fixture under tests/fixtures/fixable/per-rule/.
// This is the roadmap/17 Phase 4 coverage gate: new fixable rules
// must land with a fixture, or the rule should mark itself
// IsFixable() = false until a fixture exists.
func TestFixableRulesHavePerRuleFixture(t *testing.T) {
	root := fixtureRoot(t)
	perRuleDir := filepath.Join(root, "fixable", "per-rule")

	existing := make(map[string]bool)
	entries, err := os.ReadDir(perRuleDir)
	if err != nil {
		t.Fatalf("reading per-rule dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".kt") {
			continue
		}
		expectedPath := filepath.Join(perRuleDir, e.Name()+".expected")
		if _, err := os.Stat(expectedPath); err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".kt")
		existing[name] = true
	}

	var missing []string
	for _, r := range v2rules.Registry {
		if r.Fix == v2rules.FixNone {
			continue
		}
		if !existing[r.ID] {
			missing = append(missing, r.ID)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("fixable rules without a per-rule fixture (%d):\n  %s\n\nAdd tests/fixtures/fixable/per-rule/<RuleName>.kt and run with UPDATE_FIXABLE_EXPECTED=1 to bootstrap the expected file. If the rule cannot produce a fix on any input, change its IsFixable() return to false.",
			len(missing), strings.Join(missing, "\n  "))
	}
}
