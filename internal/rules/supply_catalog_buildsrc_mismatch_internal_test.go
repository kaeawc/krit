package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestVersionCatalogBuildSrcMismatch(t *testing.T) {
	root := internalFixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "version-catalog-build-src-mismatch")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "version-catalog-build-src-mismatch")

	t.Run("positive fixture flags every conflicting alias", func(t *testing.T) {
		findings := runVersionCatalogBuildSrcMismatch(t, positiveDir)
		seen := map[string]bool{}
		for _, f := range findings {
			for _, alias := range []string{"okhttp", "gson"} {
				if strings.Contains(f.Message, "'"+alias+"'") {
					seen[alias] = true
				}
			}
			if strings.Contains(f.Message, "'retrofit'") {
				t.Errorf("retrofit version matches the catalog and must not be flagged: %q", f.Message)
			}
		}
		for _, alias := range []string{"okhttp", "gson"} {
			if !seen[alias] {
				t.Errorf("expected finding for alias %q; got %d findings: %v", alias, len(findings), findings)
			}
		}
		if len(findings) != 2 {
			t.Errorf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runVersionCatalogBuildSrcMismatch(t, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestReadCatalogCoordinates(t *testing.T) {
	cases := []struct {
		name, src string
		wantLine  int
	}{
		{
			name:     "library sub-table",
			src:      "[versions]\nk = \"1.0\"\n[libraries.core]\nmodule = \"a:b\"\nversion.ref = \"k\"\n",
			wantLine: 3,
		},
		{
			name:     "inline version table",
			src:      "[versions]\nk = \"1.0\"\n[libraries]\ncore = { module = \"a:b\", version = { ref = \"k\" } }\n",
			wantLine: 4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "libs.versions.toml")
			if err := os.WriteFile(path, []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}
			cat, err := module.ParseVersionCatalog(path)
			if err != nil {
				t.Fatal(err)
			}
			coords := readCatalogCoordinates(cat)
			got, ok := coords["a:b"]
			if !ok {
				t.Fatalf("coordinate a:b missing: %+v", coords)
			}
			if got.Alias != "core" || got.Version != "1.0" {
				t.Errorf("coordinate = %+v, want alias core and version 1.0", got)
			}
			if got.Line != tc.wantLine {
				t.Errorf("coordinate line = %d, want %d", got.Line, tc.wantLine)
			}
		})
	}
}

func runVersionCatalogBuildSrcMismatch(t *testing.T, projectDir string) []scanner.Finding {
	t.Helper()
	graph, err := module.DiscoverModules(t.Context(), projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}
	rule := &VersionCatalogBuildSrcMismatchRule{
		BaseRule: BaseRule{RuleName: "VersionCatalogBuildSrcMismatch", RuleSetName: supplyChainRuleSet, Sev: "warning"},
	}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	return api.ContextFindings(ctx)
}
