package rules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestVersionCatalogDuplicateVersion(t *testing.T) {
	root := internalFixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "version-catalog-duplicate-version")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "version-catalog-duplicate-version")

	t.Run("positive fixture flags duplicate literals", func(t *testing.T) {
		findings := runVersionCatalogDuplicateVersion(t, positiveDir)
		// Two duplicate groups (4.12.0 and 1.9.0) → one finding each on
		// the secondary alias.
		expected := map[string]string{
			"retrofit": "okhttp",
			"compose":  "kotlin",
		}
		seen := map[string]bool{}
		for _, f := range findings {
			for secondary, primary := range expected {
				if strings.Contains(f.Message, "'"+secondary+"'") && strings.Contains(f.Message, "'"+primary+"'") {
					seen[secondary] = true
				}
			}
		}
		for secondary := range expected {
			if !seen[secondary] {
				t.Errorf("expected duplicate finding for alias %q; got %d findings: %v", secondary, len(findings), findings)
			}
		}
		if len(findings) != len(expected) {
			t.Errorf("expected %d findings, got %d: %v", len(expected), len(findings), findings)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runVersionCatalogDuplicateVersion(t, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func runVersionCatalogDuplicateVersion(t *testing.T, projectDir string) []scanner.Finding {
	t.Helper()
	graph, err := module.DiscoverModules(t.Context(), projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}
	rule := &VersionCatalogDuplicateVersionRule{
		BaseRule: BaseRule{RuleName: "VersionCatalogDuplicateVersion", RuleSetName: supplyChainRuleSet, Sev: "warning"},
	}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	return api.ContextFindings(ctx)
}
