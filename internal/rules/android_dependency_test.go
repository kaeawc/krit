package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

func TestAndroidDependencyMetadata(t *testing.T) {
	check := func(name string, want rules.AndroidDataDependency) {
		t.Helper()
		rule := findRegisteredRule(t, name)
		if got := rules.AndroidDependenciesOf(rule); got != want {
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
	check("IconDensities", rules.AndroidDepIcons)
	check("UseValueOf", rules.AndroidDepNone)
}

func findRegisteredRule(t *testing.T, name string) rules.Rule {
	t.Helper()
	for _, r := range rules.Registry {
		if r.Name() == name {
			return r
		}
	}
	t.Fatalf("rule %q not found in registry", name)
	return nil
}
