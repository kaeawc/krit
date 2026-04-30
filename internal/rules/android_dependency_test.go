package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

func TestAndroidDependencyMetadata(t *testing.T) {
	check := func(name string, want rules.AndroidDataDependency) {
		t.Helper()
		rule := findRegisteredRule(t, name)
		if got := rules.AndroidDataDependency(rule.AndroidDeps); got != want {
			t.Fatalf("rule %q dependencies = %v, want %v", name, got, want)
		}
	}

	check("AllowBackupManifest", rules.AndroidDepManifest)
	check("LocaleConfigMissing", rules.AndroidDepManifest)
	check("LocaleConfigStale", rules.AndroidDepValues)
	check("HardcodedValuesResource", rules.AndroidDepLayout)
	check("MissingQuantityResource", rules.AndroidDepValuesPlurals)
	check("StringFormatInvalidResource", rules.AndroidDepValuesStrings)
	check("InconsistentArraysResource", rules.AndroidDepValuesArrays)
	check("ExtraTextResource", rules.AndroidDepValuesExtraText)
	check("PxUsageResource", rules.AndroidDepValuesDimensions|rules.AndroidDepLayout)
	check("GradlePluginCompatibility", rules.AndroidDepGradle)
	for _, name := range allIconRuleNames() {
		check(name, rules.AndroidDepIcons)
	}
	check("UseValueOf", rules.AndroidDepNone)
}

func TestIconRulesAreClassifiedForV2Dispatch(t *testing.T) {
	dispatcher := rules.NewDispatcherV2(v2rules.Registry)
	got := make(map[string]bool)
	for _, rule := range dispatcher.IconRules() {
		if rule.Check == nil {
			t.Fatalf("icon rule %q has nil Check", rule.ID)
		}
		got[rule.ID] = true
	}
	for _, name := range allIconRuleNames() {
		if !got[name] {
			t.Fatalf("icon rule %q was not classified as an icon rule", name)
		}
	}
}

func allIconRuleNames() []string {
	return []string{
		"IconDensities",
		"IconDipSize",
		"IconDuplicates",
		"GifUsage",
		"ConvertToWebp",
		"IconMissingDensityFolder",
		"IconExpectedSize",
		"IconExtension",
		"IconLocation",
		"IconMixedNinePatch",
		"IconXmlAndPng",
		"IconNoDpi",
		"IconDuplicatesConfig",
		"IconColors",
		"IconLauncherShape",
	}
}

func findRegisteredRule(t *testing.T, name string) *v2rules.Rule {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.ID == name {
			return r
		}
	}
	t.Fatalf("rule %q not found in registry", name)
	return nil
}
