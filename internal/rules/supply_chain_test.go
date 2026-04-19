package rules_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestCompileSdkMismatchAcrossModules(t *testing.T) {
	registered := buildRuleIndex()["CompileSdkMismatchAcrossModules"]
	if registered == nil {
		t.Fatal("CompileSdkMismatchAcrossModules rule not registered")
	}

	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "compile-sdk-mismatch-across-modules")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "compile-sdk-mismatch-across-modules")

	t.Run("positive fixture triggers", func(t *testing.T) {
		findings := runCompileSdkMismatchAcrossModulesRule(t, positiveDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, ":feature:a=33") || !strings.Contains(findings[0].Message, ":feature:b=34") {
			t.Fatalf("expected finding to summarize both module compileSdk values, got %q", findings[0].Message)
		}
		if !strings.Contains(findings[0].Message, "Module :feature:a declares compileSdk 33") {
			t.Fatalf("expected finding to point at the lower compileSdk module, got %q", findings[0].Message)
		}
		if !strings.HasSuffix(filepath.ToSlash(findings[0].File), "/feature/a/build.gradle.kts") {
			t.Fatalf("expected finding to point at feature/a build file, got %q", findings[0].File)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runCompileSdkMismatchAcrossModulesRule(t, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runCompileSdkMismatchAcrossModulesRule(t *testing.T, projectDir string) []scanner.Finding {
	t.Helper()

	graph, err := module.DiscoverModules(projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}

	rule := buildRuleIndex()["CompileSdkMismatchAcrossModules"]
	if rule == nil {
		t.Fatal("CompileSdkMismatchAcrossModules not registered")
	}

	ctx := &v2.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.Check(ctx)
	return v2.ContextFindings(ctx)
}
