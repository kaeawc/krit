package rules

import (
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

func TestParseLibraryRHS(t *testing.T) {
	versions := map[string]string{"okhttp": "4.12.0"}
	cases := []struct {
		name        string
		value       string
		wantModule  string
		wantVersion string
	}{
		{"shorthand", `"com.example:lib:1.2.3"`, "com.example:lib", "1.2.3"},
		{"inline-literal", `{ module = "g:n", version = "9.9" }`, "g:n", "9.9"},
		{"inline-ref", `{ module = "com.squareup.okhttp3:okhttp", version.ref = "okhttp" }`, "com.squareup.okhttp3:okhttp", "4.12.0"},
		{"split-fields", `{ group = "g", name = "n", version = "1.0" }`, "g:n", "1.0"},
		{"unrecognized-bare", `42`, "", ""},
		{"shorthand-no-version", `"only:two"`, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMod, gotVer := parseLibraryRHS(tc.value, versions)
			if gotMod != tc.wantModule || gotVer != tc.wantVersion {
				t.Errorf("parseLibraryRHS(%q) = (%q,%q), want (%q,%q)", tc.value, gotMod, gotVer, tc.wantModule, tc.wantVersion)
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
