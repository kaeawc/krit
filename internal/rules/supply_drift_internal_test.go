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

func TestConventionPluginAppliedToWrongTarget(t *testing.T) {
	root := internalFixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "convention-plugin-applied-to-wrong-target")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "convention-plugin-applied-to-wrong-target")

	t.Run("positive fixture flags android convention on jvm module", func(t *testing.T) {
		findings := runConventionPluginAppliedToWrongTarget(t, positiveDir, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "android-library") {
			t.Fatalf("expected finding to mention plugin id, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture allows android convention on android module", func(t *testing.T) {
		findings := runConventionPluginAppliedToWrongTarget(t, negativeDir, nil)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("explicit map replaces name inference", func(t *testing.T) {
		findings := runConventionPluginAppliedToWrongTarget(t, positiveDir, []string{"android-library=any"})
		if len(findings) != 0 {
			t.Fatalf("expected explicit any target to suppress finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("root build script is not a valid android convention target", func(t *testing.T) {
		dir := t.TempDir()
		writeSupplyDriftFile(t, filepath.Join(dir, "settings.gradle.kts"), `rootProject.name = "root-target"`)
		writeSupplyDriftFile(t, filepath.Join(dir, "build.gradle.kts"), `plugins {
    id("android-library")
}
`)
		findings := runConventionPluginAppliedToWrongTarget(t, dir, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 root finding, got %d: %v", len(findings), findings)
		}
	})
}

func runConventionPluginAppliedToWrongTarget(t *testing.T, root string, pluginTargetMap []string) []scanner.Finding {
	t.Helper()
	graph, err := module.DiscoverModules(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if graph == nil {
		graph = module.NewModuleGraph(root)
	}
	rule := &ConventionPluginAppliedToWrongTargetRule{
		BaseRule:        BaseRule{RuleName: "ConventionPluginAppliedToWrongTarget", RuleSetName: supplyChainRuleSet, Sev: "warning"},
		PluginTargetMap: pluginTargetMap,
	}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	return api.ContextFindings(ctx)
}

func writeSupplyDriftFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
