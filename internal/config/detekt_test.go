package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeView is a small registry stand-in for the importer.
type fakeView struct {
	rules   map[string]string            // rule -> ruleset
	options map[string]map[string]string // rule -> key or alias -> primary
}

func (f fakeView) HasRuleSet(set string) bool {
	for _, s := range f.rules {
		if s == set {
			return true
		}
	}
	return false
}

func (f fakeView) RuleSetOf(rule string) (string, bool) {
	s, ok := f.rules[rule]
	return s, ok
}

func (f fakeView) CanonicalOption(rule, key string) (string, bool) {
	c, ok := f.options[rule][key]
	return c, ok
}

func installFakeView(t *testing.T) {
	t.Helper()
	prev := registryView
	t.Cleanup(func() { registryView = prev })
	SetRegistryView(fakeView{
		rules: map[string]string{
			"LongMethod":              "complexity",
			"NestedBlockDepth":        "complexity",
			"TooManyFunctions":        "complexity",
			"CyclomaticComplexMethod": "complexity",
			"LongParameterList":       "complexity",
			"MaxLineLength":           "style",
			"MagicNumber":             "style",
			"ReturnCount":             "style",
			"FunctionNaming":          "naming",
			"TopLevelPropertyNaming":  "naming",
			"DestructuringDeclarationWithTooManyEntries": "style",
		},
		options: map[string]map[string]string{
			"LongMethod":              {"allowedLines": "allowedLines", "threshold": "allowedLines"},
			"NestedBlockDepth":        {"allowedDepth": "allowedDepth", "threshold": "allowedDepth"},
			"TooManyFunctions":        {"allowedFunctionsPerFile": "allowedFunctionsPerFile", "ignoreDeprecated": "ignoreDeprecated"},
			"CyclomaticComplexMethod": {"allowedComplexity": "allowedComplexity", "threshold": "allowedComplexity"},
			"LongParameterList":       {"allowedFunctionParameters": "allowedFunctionParameters", "allowedConstructorParameters": "allowedConstructorParameters"},
			"MaxLineLength":           {"maxLineLength": "maxLineLength"},
			"MagicNumber":             {"ignoreNumbers": "ignoreNumbers"},
			"ReturnCount":             {"max": "max", "excludeGuardClauses": "excludeGuardClauses"},
			"TopLevelPropertyNaming":  {"constantPattern": "constantPattern"},
			"DestructuringDeclarationWithTooManyEntries": {"maxDestructuringEntries": "maxDestructuringEntries"},
		},
	})
}

func translate(t *testing.T, yml string) (*Config, []string) {
	t.Helper()
	out, notes, err := ParseDetektConfig([]byte(yml), "detekt.yml")
	if err != nil {
		t.Fatal(err)
	}
	return NewConfigFromData(out), notes
}

func TestDetektThresholdsShiftToAllowedValues(t *testing.T) {
	installFakeView(t)
	cfg, notes := translate(t, `
complexity:
  LongMethod:
    threshold: 120
  NestedBlockDepth:
    threshold: 6
  TooManyFunctions:
    thresholdInFiles: 40
    thresholdInClasses: 40
    ignoreDeprecated: true
  LongParameterList:
    functionThreshold: 9
    constructorThreshold: 11
`)
	if len(notes) != 0 {
		t.Fatalf("unexpected notes: %v", notes)
	}
	// detekt reports at >= threshold; krit reports above the allowed value.
	cases := []struct {
		set, rule, key string
		want           int
	}{
		{"complexity", "LongMethod", "allowedLines", 119},
		{"complexity", "NestedBlockDepth", "allowedDepth", 5},
		{"complexity", "TooManyFunctions", "allowedFunctionsPerFile", 39},
		{"complexity", "TooManyFunctions", "allowedFunctionsPerClass", 39},
		{"complexity", "LongParameterList", "allowedFunctionParameters", 8},
		{"complexity", "LongParameterList", "allowedConstructorParameters", 10},
	}
	for _, c := range cases {
		if got := cfg.GetInt(c.set, c.rule, c.key, -1); got != c.want {
			t.Errorf("%s.%s.%s = %d, want %d", c.set, c.rule, c.key, got, c.want)
		}
	}
	if !cfg.GetBool("complexity", "TooManyFunctions", "ignoreDeprecated", false) {
		t.Error("ignoreDeprecated should pass through")
	}
}

func TestDetektActiveAndPassThroughOptions(t *testing.T) {
	installFakeView(t)
	cfg, notes := translate(t, `
style:
  MaxLineLength:
    active: false
  DestructuringDeclarationWithTooManyEntries:
    active: false
  ReturnCount:
    active: true
    max: 2
    excludeGuardClauses: true
  MagicNumber:
    ignoreNumbers: ['-1', '0']
    ignoreAnnotated: ['Preview']
    excludes: ['**/test/**']
    includes: ['**/main/**']
naming:
  TopLevelPropertyNaming:
    constantPattern: '[A-Z][A-Z0-9_]*'
`)
	if len(notes) != 0 {
		t.Fatalf("unexpected notes: %v", notes)
	}
	for _, rule := range []string{"MaxLineLength", "DestructuringDeclarationWithTooManyEntries"} {
		if a := cfg.IsRuleActive("style", rule); a == nil || *a {
			t.Errorf("%s should be inactive", rule)
		}
	}
	if got := cfg.GetInt("style", "ReturnCount", "max", 0); got != 2 {
		t.Errorf("ReturnCount.max = %d, want 2 (no shift for max-style options)", got)
	}
	if got := cfg.GetStringList("style", "MagicNumber", "ignoreAnnotated"); len(got) != 1 || got[0] != "Preview" {
		t.Errorf("ignoreAnnotated = %v", got)
	}
	if got := cfg.GetStringList("style", "MagicNumber", "excludes"); len(got) != 1 {
		t.Errorf("excludes = %v", got)
	}
	if got := cfg.GetStringList("style", "MagicNumber", "includes"); len(got) != 1 {
		t.Errorf("includes = %v", got)
	}
	if got := cfg.GetString("naming", "TopLevelPropertyNaming", "constantPattern", ""); got != "[A-Z][A-Z0-9_]*" {
		t.Errorf("constantPattern = %q", got)
	}
}

func TestDetektRuleRenames(t *testing.T) {
	installFakeView(t)
	cfg, notes := translate(t, `
complexity:
  ComplexMethod:
    threshold: 15
naming:
  FunctionName:
    active: false
`)
	if len(notes) != 0 {
		t.Fatalf("unexpected notes: %v", notes)
	}
	if got := cfg.GetInt("complexity", "CyclomaticComplexMethod", "allowedComplexity", -1); got != 14 {
		t.Errorf("CyclomaticComplexMethod.allowedComplexity = %d, want 14", got)
	}
	if a := cfg.IsRuleActive("naming", "FunctionNaming"); a == nil || *a {
		t.Error("FunctionName: active false should disable FunctionNaming")
	}
}

func TestDetektRenamedNameLosesToKritName(t *testing.T) {
	installFakeView(t)
	cfg, _ := translate(t, `
complexity:
  ComplexMethod:
    threshold: 15
  CyclomaticComplexMethod:
    allowedComplexity: 30
`)
	if got := cfg.GetInt("complexity", "CyclomaticComplexMethod", "allowedComplexity", -1); got != 30 {
		t.Errorf("allowedComplexity = %d, want 30", got)
	}
}

func TestDetektRulesetActiveAndExcludes(t *testing.T) {
	installFakeView(t)
	cfg, _ := translate(t, `
complexity:
  active: false
  excludes: ['**/generated/**']
`)
	if a := cfg.IsRuleSetActive("complexity"); a == nil || *a {
		t.Error("complexity ruleset should be inactive")
	}
	if got := cfg.GetTopLevelStringList("complexity", "excludes"); len(got) != 1 {
		t.Errorf("ruleset excludes = %v", got)
	}
}

func TestDetektUnknownKeysNoted(t *testing.T) {
	installFakeView(t)
	cfg, notes := translate(t, `
build:
  maxIssues: 0
formatting:
  MaximumLineLength:
    active: true
complexity:
  LongMethod:
    bogusOption: 1
  NoSuchRule:
    active: true
`)
	joined := strings.Join(notes, "\n")
	for _, want := range []string{`"build"`, `"formatting"`, "bogusOption", "NoSuchRule"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notes missing %q:\n%s", want, joined)
		}
	}
	if cfg.Has("complexity", "LongMethod", "bogusOption") || cfg.Has("complexity", "NoSuchRule", "active") {
		t.Error("unknown keys must not reach the config")
	}
}

func TestDetektRuleMovedToKritRuleset(t *testing.T) {
	installFakeView(t)
	// MaxLineLength lives in style for krit; a misfiled detekt block follows the registry.
	cfg, _ := translate(t, `
complexity:
  MaxLineLength:
    maxLineLength: 150
`)
	if got := cfg.GetInt("style", "MaxLineLength", "maxLineLength", 0); got != 150 {
		t.Errorf("maxLineLength = %d, want 150", got)
	}
}

func TestCanonicalizeAliasesStopsDefaultShadowing(t *testing.T) {
	installFakeView(t)
	base := NewConfigFromData(map[string]interface{}{
		"complexity": map[string]interface{}{"LongMethod": map[string]interface{}{"allowedLines": 60}},
	})
	user := map[string]interface{}{
		"complexity": map[string]interface{}{"LongMethod": map[string]interface{}{"threshold": 90}},
	}
	canonicalizeAliases(user)
	merged := &Config{data: mergeMaps(base.data, user)}
	if got := merged.GetInt("complexity", "LongMethod", "allowedLines", 0); got != 90 {
		t.Errorf("allowedLines = %d, want 90", got)
	}
	if merged.Has("complexity", "LongMethod", "threshold") {
		t.Error("alias key should be rewritten")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const detektLongMethod120 = "complexity:\n  LongMethod:\n    threshold: 120\n"

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func loadWith(t *testing.T, opts LoadOptions, userPath string, roots ...string) *Config {
	t.Helper()
	base := NewConfigFromData(map[string]interface{}{
		"complexity": map[string]interface{}{"LongMethod": map[string]interface{}{"allowedLines": 60}},
	})
	cfg, err := mergeUserConfig(base, userPath, roots, opts)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestDetektAutoDiscovery(t *testing.T) {
	installFakeView(t)
	for _, rel := range DetektConfigFilenames {
		t.Run(rel, func(t *testing.T) {
			root := newProject(t)
			writeFile(t, filepath.Join(root, rel), detektLongMethod120)
			sub := filepath.Join(root, "app", "src")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			cfg := loadWith(t, LoadOptions{}, "", sub)
			if got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 0); got != 119 {
				t.Errorf("allowedLines = %d, want 119", got)
			}
			if d := cfg.Detekt(); d == nil || d.Source != filepath.Join(root, rel) {
				t.Errorf("Detekt() = %+v", d)
			}
		})
	}
}

func TestDetektNotDiscoveredBeyondVCSRoot(t *testing.T) {
	installFakeView(t)
	outer := t.TempDir()
	writeFile(t, filepath.Join(outer, "detekt.yml"), detektLongMethod120)
	inner := filepath.Join(outer, "repo")
	if err := os.MkdirAll(filepath.Join(inner, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := loadWith(t, LoadOptions{}, "", inner)
	if cfg.Detekt() != nil || cfg.GetInt("complexity", "LongMethod", "allowedLines", 0) != 60 {
		t.Error("detekt config above the VCS root must not be imported")
	}
}

func TestKritYmlOverridesDetekt(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	writeFile(t, filepath.Join(root, "detekt.yml"), `
complexity:
  LongMethod:
    threshold: 120
  NestedBlockDepth:
    threshold: 6
style:
  MaxLineLength:
    active: false
`)
	writeFile(t, filepath.Join(root, "krit.yml"), `
complexity:
  LongMethod:
    threshold: 90
style:
  MaxLineLength:
    active: true
`)
	cfg := loadWith(t, LoadOptions{}, "", root)
	// krit.yml's alias value (90, not shifted) beats detekt's 120 despite the primary key in defaults.
	if got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 0); got != 90 {
		t.Errorf("LongMethod.allowedLines = %d, want 90 from krit.yml", got)
	}
	if got := cfg.GetInt("complexity", "NestedBlockDepth", "allowedDepth", 0); got != 5 {
		t.Errorf("NestedBlockDepth.allowedDepth = %d, want 5 from detekt", got)
	}
	if a := cfg.IsRuleActive("style", "MaxLineLength"); a == nil || !*a {
		t.Error("krit.yml active: true should beat detekt active: false")
	}
}

func TestDetektOptOut(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	writeFile(t, filepath.Join(root, "detekt.yml"), detektLongMethod120)

	t.Run("option", func(t *testing.T) {
		cfg := loadWith(t, LoadOptions{NoDetekt: true}, "", root)
		if cfg.Detekt() != nil || cfg.GetInt("complexity", "LongMethod", "allowedLines", 0) != 60 {
			t.Error("NoDetekt must skip import")
		}
	})
	t.Run("krit.yml", func(t *testing.T) {
		writeFile(t, filepath.Join(root, "krit.yml"), "detekt:\n  enabled: false\n")
		defer os.Remove(filepath.Join(root, "krit.yml"))
		cfg := loadWith(t, LoadOptions{}, "", root)
		if cfg.Detekt() != nil || cfg.GetInt("complexity", "LongMethod", "allowedLines", 0) != 60 {
			t.Error("detekt.enabled: false must skip import")
		}
	})
	t.Run("explicit file beats enabled false", func(t *testing.T) {
		writeFile(t, filepath.Join(root, "krit.yml"), "detekt:\n  enabled: false\n")
		defer os.Remove(filepath.Join(root, "krit.yml"))
		cfg := loadWith(t, LoadOptions{DetektConfigFile: filepath.Join(root, "detekt.yml")}, "", root)
		if cfg.GetInt("complexity", "LongMethod", "allowedLines", 0) != 119 {
			t.Error("--detekt-config should import even when krit.yml disables discovery")
		}
	})
}

func TestDetektConfigFileSetting(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	writeFile(t, filepath.Join(root, "tools", "lint.yml"), detektLongMethod120)
	writeFile(t, filepath.Join(root, "krit.yml"), "detekt:\n  configFile: tools/lint.yml\n")
	cfg := loadWith(t, LoadOptions{}, "", root)
	if got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 0); got != 119 {
		t.Errorf("allowedLines = %d, want 119", got)
	}
}

func TestDetektMissingExplicitFileIsError(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	base := NewConfig()
	_, err := mergeUserConfig(base, "", []string{root}, LoadOptions{DetektConfigFile: filepath.Join(root, "missing.yml")})
	if err == nil {
		t.Fatal("expected an error for a missing explicit detekt config")
	}
}

func TestDetektBaselineDiscovery(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	writeFile(t, filepath.Join(root, "config", "detekt", "detekt.yml"), detektLongMethod120)
	writeFile(t, filepath.Join(root, "config", "detekt", "baseline.xml"), "<SmellBaseline/>")
	cfg := loadWith(t, LoadOptions{}, "", root)
	want := filepath.Join(root, "config", "detekt", "baseline.xml")
	if d := cfg.Detekt(); d == nil || d.Baseline != want {
		t.Errorf("Detekt() = %+v, want baseline %s", d, want)
	}

	writeFile(t, filepath.Join(root, "krit.yml"), "detekt:\n  baseline: other.xml\n")
	cfg = loadWith(t, LoadOptions{}, "", root)
	if d := cfg.Detekt(); d == nil || d.Baseline != filepath.Join(root, "other.xml") {
		t.Errorf("explicit baseline not honoured: %+v", cfg.Detekt())
	}
}

func TestDetektSummary(t *testing.T) {
	d := &DetektImport{Source: "a/detekt.yml", Baseline: "a/baseline.xml", Notes: []string{"x ignored"}}
	got := strings.Join(d.Summary(), "|")
	for _, want := range []string{"a/detekt.yml", "a/baseline.xml", "x ignored"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
	if (*DetektImport)(nil).Summary() != nil {
		t.Error("nil import has no summary")
	}
}

func TestDetektInvalidYAML(t *testing.T) {
	if _, _, err := ParseDetektConfig([]byte("complexity: [unterminated"), "bad.yml"); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestDetektDefaultsApplyBeneathProjectConfig(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	writeFile(t, filepath.Join(root, "detekt.yml"), `
style:
  ReturnCount:
    max: 4
`)
	base := NewConfigFromData(map[string]interface{}{
		"style": map[string]interface{}{"ReturnCount": map[string]interface{}{"max": 3, "excludeGuardClauses": true}},
	})
	cfg, err := mergeUserConfig(base, "", []string{root}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetInt("style", "ReturnCount", "max", 0); got != 4 {
		t.Errorf("ReturnCount.max = %d, want 4 from the project", got)
	}
	// Left out of the project config, so detekt's default applies instead of krit's.
	if got := cfg.GetBool("style", "ReturnCount", "excludeGuardClauses", true); got {
		t.Error("ReturnCount.excludeGuardClauses should take detekt's default (false)")
	}
}

func TestDetektDefaultsNotAppliedWithoutDetektConfig(t *testing.T) {
	installFakeView(t)
	root := newProject(t)
	base := NewConfigFromData(map[string]interface{}{
		"style": map[string]interface{}{"ReturnCount": map[string]interface{}{"excludeGuardClauses": true}},
	})
	cfg, err := mergeUserConfig(base, "", []string{root}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Detekt() != nil {
		t.Fatal("no detekt config expected")
	}
	if got := cfg.GetBool("style", "ReturnCount", "excludeGuardClauses", false); !got {
		t.Error("krit's own default should stand when no detekt config is imported")
	}
}
