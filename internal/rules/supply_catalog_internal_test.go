package rules

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestVersionCatalogUnused(t *testing.T) {
	root := internalFixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "version-catalog-unused")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "version-catalog-unused")

	t.Run("positive fixture flags unused aliases", func(t *testing.T) {
		findings := runVersionCatalogUnused(t, positiveDir, nil, true)
		expected := map[string]bool{
			"unused-lib":       false,
			"forgotten-plugin": false,
			"ghost-bundle":     false,
		}
		for _, f := range findings {
			for alias := range expected {
				if strings.Contains(f.Message, "'"+alias+"'") {
					expected[alias] = true
				}
			}
		}
		for alias, seen := range expected {
			if !seen {
				t.Errorf("expected finding for unused alias %q; got %d findings: %v", alias, len(findings), findings)
			}
		}
		if len(findings) != len(expected) {
			t.Errorf("expected exactly %d findings, got %d: %v", len(expected), len(findings), findings)
		}
	})

	t.Run("negative fixture is clean when convention plugins are scanned", func(t *testing.T) {
		findings := runVersionCatalogUnused(t, negativeDir, nil, true)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("convention-only alias flagged when scanConventionPlugins=false", func(t *testing.T) {
		findings := runVersionCatalogUnused(t, negativeDir, nil, false)
		matched := false
		for _, f := range findings {
			if strings.Contains(f.Message, "'convention-helper'") {
				matched = true
			}
		}
		if !matched {
			t.Fatalf("expected convention-helper to be flagged when scanConventionPlugins=false; got %v", findings)
		}
	})

	t.Run("ignoredAliases suppresses matching findings", func(t *testing.T) {
		findings := runVersionCatalogUnused(t, positiveDir, []string{"unused-*", "ghost-bundle"}, true)
		for _, f := range findings {
			if strings.Contains(f.Message, "'unused-lib'") || strings.Contains(f.Message, "'ghost-bundle'") {
				t.Fatalf("expected ignoredAliases to suppress unused-lib and ghost-bundle; got finding %q", f.Message)
			}
		}
	})
}

func TestMatchAliasGlob(t *testing.T) {
	cases := []struct {
		pattern, alias string
		want           bool
	}{
		{"foo", "foo", true},
		{"foo", "foobar", false},
		{"foo-*", "foo-bar", true},
		{"foo-*", "bar-foo", false},
		{"*-foo", "bar-foo", true},
		{"*plugin*", "convention-plugin-x", true},
		{"plain*", "different", false},
	}
	for _, c := range cases {
		if got := matchAliasGlob(c.pattern, c.alias); got != c.want {
			t.Errorf("matchAliasGlob(%q, %q) = %v, want %v", c.pattern, c.alias, got, c.want)
		}
	}
}

func TestAccessorReferencedRespectsBoundaries(t *testing.T) {
	corpus := "implementation(libs.okhttp.core)\nval s = \"libs.okhttpExtra\"\n"
	if !accessorReferenced(corpus, "libs.okhttp.core") {
		t.Error("expected exact accessor to match")
	}
	if accessorReferenced(corpus, "libs.okhttp") {
		t.Error("did not expect libs.okhttp to match libs.okhttpExtra or libs.okhttp.core")
	}
}

func runVersionCatalogUnused(t *testing.T, projectDir string, ignored []string, scanConvention bool) []scanner.Finding {
	t.Helper()
	graph, err := module.DiscoverModules(projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}
	rule := &VersionCatalogUnusedRule{
		BaseRule:              BaseRule{RuleName: "VersionCatalogUnused", RuleSetName: supplyChainRuleSet, Sev: "info"},
		IgnoredAliases:        ignored,
		ScanConventionPlugins: scanConvention,
	}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	return api.ContextFindings(ctx)
}

func internalFixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "tests", "fixtures")
}
