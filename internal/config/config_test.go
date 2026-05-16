package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")
	content := `
complexity:
  LongMethod:
    active: true
    allowedLines: 120
  NestedBlockDepth:
    active: true
    threshold: 6
naming:
  FunctionNaming:
    active: true
    ignoreAnnotated:
      - 'Composable'
style:
  MagicNumber:
    active: false
    ignoreAnnotated:
      - 'Preview'
    ignoreNumbers:
      - '-1'
      - '0'
      - '1'
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	// Test IsRuleActive
	active := cfg.IsRuleActive("style", "MagicNumber")
	if active == nil || *active != false {
		t.Errorf("expected MagicNumber active=false, got %v", active)
	}

	active = cfg.IsRuleActive("complexity", "LongMethod")
	if active == nil || *active != true {
		t.Errorf("expected LongMethod active=true, got %v", active)
	}

	// Test not-specified rule
	active = cfg.IsRuleActive("style", "NonExistent")
	if active != nil {
		t.Errorf("expected nil for non-existent rule, got %v", *active)
	}

	// Test GetInt
	val := cfg.GetInt("complexity", "LongMethod", "allowedLines", 60)
	if val != 120 {
		t.Errorf("expected allowedLines=120, got %d", val)
	}

	val = cfg.GetInt("complexity", "NestedBlockDepth", "threshold", 4)
	if val != 6 {
		t.Errorf("expected threshold=6, got %d", val)
	}

	// Test GetStringList
	annotations := cfg.GetStringList("naming", "FunctionNaming", "ignoreAnnotated")
	if len(annotations) != 1 || annotations[0] != "Composable" {
		t.Errorf("expected [Composable], got %v", annotations)
	}

	ignoreNums := cfg.GetStringList("style", "MagicNumber", "ignoreNumbers")
	if len(ignoreNums) != 3 {
		t.Errorf("expected 3 ignoreNumbers, got %d", len(ignoreNums))
	}
}

func TestMergeMaps(t *testing.T) {
	dst := map[string]interface{}{
		"complexity": map[string]interface{}{
			"LongMethod": map[string]interface{}{
				"active":       true,
				"allowedLines": 60,
			},
		},
	}
	src := map[string]interface{}{
		"complexity": map[string]interface{}{
			"LongMethod": map[string]interface{}{
				"allowedLines": 120,
			},
		},
		"style": map[string]interface{}{
			"MagicNumber": map[string]interface{}{
				"active": false,
			},
		},
	}

	result := mergeMaps(dst, src)

	// LongMethod should have merged values
	complexity := result["complexity"].(map[string]interface{})
	lm := complexity["LongMethod"].(map[string]interface{})
	if lm["active"] != true {
		t.Error("expected active=true preserved from dst")
	}
	if lm["allowedLines"] != 120 {
		t.Error("expected allowedLines=120 from src override")
	}

	// style should be added from src
	style := result["style"].(map[string]interface{})
	mn := style["MagicNumber"].(map[string]interface{})
	if mn["active"] != false {
		t.Error("expected MagicNumber active=false from src")
	}
}

func TestLoadConfigExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yml")
	content := `
style:
  WildcardImport:
    active: true
    excludeImports:
      - 'kotlinx.coroutines.*'
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	active := cfg.IsRuleActive("style", "WildcardImport")
	if active == nil || *active != true {
		t.Errorf("expected WildcardImport active=true, got %v", active)
	}
}

func TestLoadConfigMissingPath(t *testing.T) {
	_, err := LoadConfig("/no/such/path/missing.yml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadConfigEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected no error for empty YAML, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for empty YAML")
	}
	// Should behave like empty config
	if cfg.IsRuleActive("x", "y") != nil {
		t.Error("expected nil for empty config rule lookup")
	}
}

func TestFindDefaultConfigNoFile(t *testing.T) {
	// Save and restore working directory
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	// Change to a temp directory where no config/default-krit.yml exists
	dir := t.TempDir()
	os.Chdir(dir)

	result := FindDefaultConfig()
	if result != "" {
		t.Errorf("expected empty string when no default config exists, got %q", result)
	}
}

func TestGetBoolMissingKey(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("style", "MagicNumber", "active", true)

	got := cfg.GetBool("style", "MagicNumber", "nonexistent", false)
	if got != false {
		t.Errorf("expected default false for missing key, got %v", got)
	}
}

func TestGetBoolPresentKey(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("style", "MagicNumber", "active", true)

	got := cfg.GetBool("style", "MagicNumber", "active", false)
	if got != true {
		t.Errorf("expected true, got %v", got)
	}
}

func TestGetBoolWrongType(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("style", "MagicNumber", "active", 42) // int, not bool

	got := cfg.GetBool("style", "MagicNumber", "active", true)
	if got != true {
		t.Errorf("expected default true for wrong type, got %v", got)
	}
}

func TestGetBoolStringValues(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("style", "Rule1", "flag", "true")
	cfg.Set("style", "Rule2", "flag", "false")

	if got := cfg.GetBool("style", "Rule1", "flag", false); got != true {
		t.Errorf("expected true from string 'true', got %v", got)
	}
	if got := cfg.GetBool("style", "Rule2", "flag", true); got != false {
		t.Errorf("expected false from string 'false', got %v", got)
	}
}

func TestGetIntMissingKey(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("complexity", "LongMethod", "allowedLines", 100)

	got := cfg.GetInt("complexity", "LongMethod", "nonexistent", 50)
	if got != 50 {
		t.Errorf("expected default 50 for missing key, got %d", got)
	}
}

func TestGetIntPresentKey(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("complexity", "LongMethod", "allowedLines", 100)

	got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 50)
	if got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestGetIntStringValue(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("complexity", "LongMethod", "allowedLines", "200")

	got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 50)
	if got != 200 {
		t.Errorf("expected 200 parsed from string, got %d", got)
	}
}

func TestGetIntWrongType(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("complexity", "LongMethod", "allowedLines", "notanumber")

	got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 50)
	if got != 50 {
		t.Errorf("expected default 50 for unparseable string, got %d", got)
	}
}

func TestGetStringMissingKey(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("naming", "FunctionNaming", "pattern", "[a-z]+")

	got := cfg.GetString("naming", "FunctionNaming", "nonexistent", "default")
	if got != "default" {
		t.Errorf("expected 'default' for missing key, got %q", got)
	}
}

func TestGetStringPresentKey(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("naming", "FunctionNaming", "pattern", "[a-z]+")

	got := cfg.GetString("naming", "FunctionNaming", "pattern", "default")
	if got != "[a-z]+" {
		t.Errorf("expected '[a-z]+', got %q", got)
	}
}

func TestGetStringMissingRuleSet(t *testing.T) {
	cfg := NewConfig()

	got := cfg.GetString("nonexistent", "Rule", "key", "fallback")
	if got != "fallback" {
		t.Errorf("expected 'fallback' for missing ruleset, got %q", got)
	}
}

func TestGetStringNestedFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested.yml")
	content := `
naming:
  FunctionNaming:
    pattern: "[a-z][a-zA-Z0-9]*"
  ClassNaming:
    pattern: "[A-Z][a-zA-Z0-9]*"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	got := cfg.GetString("naming", "FunctionNaming", "pattern", "")
	if got != "[a-z][a-zA-Z0-9]*" {
		t.Errorf("expected function pattern, got %q", got)
	}
	got = cfg.GetString("naming", "ClassNaming", "pattern", "")
	if got != "[A-Z][a-zA-Z0-9]*" {
		t.Errorf("expected class pattern, got %q", got)
	}
}

func TestGetStringListMissingKey(t *testing.T) {
	cfg := NewConfig()

	got := cfg.GetStringList("style", "MagicNumber", "ignoreNumbers")
	if got != nil {
		t.Errorf("expected nil for missing key, got %v", got)
	}
}

func TestGetStringListPresentKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.yml")
	content := `
style:
  MagicNumber:
    ignoreNumbers:
      - '-1'
      - '0'
      - '1'
      - '2'
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	got := cfg.GetStringList("style", "MagicNumber", "ignoreNumbers")
	if len(got) != 4 {
		t.Fatalf("expected 4 items, got %d: %v", len(got), got)
	}
	expected := []string{"-1", "0", "1", "2"}
	for i, v := range expected {
		if got[i] != v {
			t.Errorf("item %d: expected %q, got %q", i, v, got[i])
		}
	}
}

func TestGetStringListMissingRuleSet(t *testing.T) {
	cfg := NewConfig()

	got := cfg.GetStringList("nonexistent", "Rule", "key")
	if got != nil {
		t.Errorf("expected nil for missing ruleset, got %v", got)
	}
}

func TestNilConfig(t *testing.T) {
	var cfg *Config

	// All methods should handle nil gracefully
	if cfg.IsRuleActive("x", "y") != nil {
		t.Error("expected nil from nil config")
	}
	if cfg.GetInt("x", "y", "z", 42) != 42 {
		t.Error("expected default from nil config")
	}
	if cfg.GetBool("x", "y", "z", true) != true {
		t.Error("expected default from nil config")
	}
	if cfg.GetString("x", "y", "z", "def") != "def" {
		t.Error("expected default from nil config")
	}
	if cfg.GetStringList("x", "y", "z") != nil {
		t.Error("expected nil from nil config")
	}
}

func TestLoadConfigMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	os.WriteFile(path, []byte("key: [unclosed\ninvalid: : :"), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestLoadAndMergeUserOverridesDefaults(t *testing.T) {
	dir := t.TempDir()

	defaultPath := filepath.Join(dir, "defaults.yml")
	os.WriteFile(defaultPath, []byte(`
complexity:
  LongMethod:
    active: true
    allowedLines: 60
  NestedBlockDepth:
    active: true
    threshold: 4
`), 0644)

	userPath := filepath.Join(dir, "user.yml")
	os.WriteFile(userPath, []byte(`
complexity:
  LongMethod:
    allowedLines: 120
  NestedBlockDepth:
    active: false
`), 0644)

	cfg, err := LoadAndMerge(userPath, defaultPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User value overrides default
	if got := cfg.GetInt("complexity", "LongMethod", "allowedLines", 0); got != 120 {
		t.Errorf("expected user override 120, got %d", got)
	}

	// Default value preserved when user doesn't specify it
	active := cfg.IsRuleActive("complexity", "LongMethod")
	if active == nil || *active != true {
		t.Errorf("expected LongMethod active=true preserved from defaults, got %v", active)
	}

	// User can deactivate a rule that defaults to active
	active = cfg.IsRuleActive("complexity", "NestedBlockDepth")
	if active == nil || *active != false {
		t.Errorf("expected NestedBlockDepth active=false from user override, got %v", active)
	}
}

func TestLoadAndMergeNoUserConfig(t *testing.T) {
	dir := t.TempDir()

	defaultPath := filepath.Join(dir, "defaults.yml")
	os.WriteFile(defaultPath, []byte(`
style:
  MaxLineLength:
    active: true
    maxLineLength: 120
`), 0644)

	// Pass a non-existent user path to trigger auto-detect which finds nothing
	// Since we're in a temp dir with no krit.yml, defaults alone are returned
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfg, err := LoadAndMerge("", defaultPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.GetInt("style", "MaxLineLength", "maxLineLength", 0); got != 120 {
		t.Errorf("expected default 120 when no user config, got %d", got)
	}
}

func TestLoadAndMergeMissingDefaultFile(t *testing.T) {
	dir := t.TempDir()

	userPath := filepath.Join(dir, "user.yml")
	os.WriteFile(userPath, []byte(`
style:
  MagicNumber:
    active: false
`), 0644)

	cfg, err := LoadAndMerge(userPath, "/no/such/defaults.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	active := cfg.IsRuleActive("style", "MagicNumber")
	if active == nil || *active != false {
		t.Errorf("expected MagicNumber active=false, got %v", active)
	}
}

func TestLoadAndMergeBadUserPath(t *testing.T) {
	_, err := LoadAndMerge("/no/such/user.yml", "")
	if err == nil {
		t.Error("expected error for missing user config file")
	}
}

func TestIsRuleSetActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yml")
	os.WriteFile(path, []byte(`
complexity:
  active: true
  LongMethod:
    active: true
naming:
  active: false
style:
  MagicNumber:
    active: true
`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	// Active ruleset
	got := cfg.IsRuleSetActive("complexity")
	if got == nil || *got != true {
		t.Errorf("expected complexity active=true, got %v", got)
	}

	// Inactive ruleset
	got = cfg.IsRuleSetActive("naming")
	if got == nil || *got != false {
		t.Errorf("expected naming active=false, got %v", got)
	}

	// Ruleset without explicit active key
	got = cfg.IsRuleSetActive("style")
	if got != nil {
		t.Errorf("expected nil for ruleset without active key, got %v", *got)
	}

	// Non-existent ruleset
	got = cfg.IsRuleSetActive("nonexistent")
	if got != nil {
		t.Errorf("expected nil for non-existent ruleset, got %v", *got)
	}

	// Nil config
	var nilCfg *Config
	got = nilCfg.IsRuleSetActive("complexity")
	if got != nil {
		t.Errorf("expected nil from nil config, got %v", *got)
	}
}

func TestGetTopLevelString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yml")
	os.WriteFile(path, []byte(`
android:
  enabled: "auto"
  sdk: "/usr/local/android"
output:
  format: sarif
  verbose: true
`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	// Existing string key
	if got := cfg.GetTopLevelString("android", "enabled", "off"); got != "auto" {
		t.Errorf("expected 'auto', got %q", got)
	}

	if got := cfg.GetTopLevelString("android", "sdk", ""); got != "/usr/local/android" {
		t.Errorf("expected '/usr/local/android', got %q", got)
	}

	// Bool value serialized as native YAML bool, should return "true"
	if got := cfg.GetTopLevelString("output", "verbose", "false"); got != "true" {
		t.Errorf("expected 'true' for bool value, got %q", got)
	}

	// Missing key returns default
	if got := cfg.GetTopLevelString("android", "missing", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}

	// Missing section returns default
	if got := cfg.GetTopLevelString("nonexistent", "key", "def"); got != "def" {
		t.Errorf("expected 'def', got %q", got)
	}

	// Nil config returns default
	var nilCfg *Config
	if got := nilCfg.GetTopLevelString("android", "enabled", "off"); got != "off" {
		t.Errorf("expected 'off' from nil config, got %q", got)
	}
}

func TestModuleTemplateConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "krit.yml")
	content := `
module_template:
  feature_root: "feature:*"
  required_submodules: [ui, data, domain]
  required_plugins:
    - com.android.library
    - org.jetbrains.kotlin.android
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := cfg.ModuleTemplate()
	if tmpl.FeatureRoot != "feature:*" {
		t.Fatalf("FeatureRoot = %q", tmpl.FeatureRoot)
	}
	if got := strings.Join(tmpl.RequiredSubmodules, ","); got != "ui,data,domain" {
		t.Fatalf("RequiredSubmodules = %q", got)
	}
	if got := strings.Join(tmpl.RequiredPlugins, ","); got != "com.android.library,org.jetbrains.kotlin.android" {
		t.Fatalf("RequiredPlugins = %q", got)
	}
}

func TestSLOs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "krit.yml")
	content := `
slos:
  - module: ":core"
    max_warnings_per_kloc: 5
  - module: ":ui"
    max_warnings_per_kloc: 10.5
    max_errors_per_kloc: 0
  - max_warnings_per_kloc: 99
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	slos := cfg.SLOs()
	if len(slos) != 2 {
		t.Fatalf("SLOs len = %d, want 2", len(slos))
	}
	if slos[0].Module != ":core" || slos[0].MaxWarningsPerKLOC == nil || *slos[0].MaxWarningsPerKLOC != 5 {
		t.Fatalf("first SLO = %#v", slos[0])
	}
	if slos[0].MaxErrorsPerKLOC != nil {
		t.Fatalf("first SLO MaxErrorsPerKLOC = %v, want nil", *slos[0].MaxErrorsPerKLOC)
	}
	if slos[1].Module != ":ui" || slos[1].MaxWarningsPerKLOC == nil || *slos[1].MaxWarningsPerKLOC != 10.5 {
		t.Fatalf("second warning SLO = %#v", slos[1])
	}
	if slos[1].MaxErrorsPerKLOC == nil || *slos[1].MaxErrorsPerKLOC != 0 {
		t.Fatalf("second error SLO = %#v", slos[1])
	}
}

func TestTestSourcePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "krit.yml")
	content := `
testSourcePaths:
  - "/src/checks/"
testSourcePathsOverride:
  - "/src/verify/"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.TestSourcePaths(), ","); got != "/src/checks/" {
		t.Fatalf("TestSourcePaths = %q", got)
	}
	if got := strings.Join(cfg.TestSourcePathsOverride(), ","); got != "/src/verify/" {
		t.Fatalf("TestSourcePathsOverride = %q", got)
	}
}

func TestGetTopLevelBool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yml")
	os.WriteFile(path, []byte(`
warningsAsErrors: true
failOnWarning: false
count: 42
`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	// Existing true value
	if got := cfg.GetTopLevelBool("warningsAsErrors", false); got != true {
		t.Errorf("expected true, got %v", got)
	}

	// Existing false value
	if got := cfg.GetTopLevelBool("failOnWarning", true); got != false {
		t.Errorf("expected false, got %v", got)
	}

	// Missing key returns default
	if got := cfg.GetTopLevelBool("nonexistent", true); got != true {
		t.Errorf("expected default true, got %v", got)
	}

	// Non-bool type returns default
	if got := cfg.GetTopLevelBool("count", false); got != false {
		t.Errorf("expected default false for int type, got %v", got)
	}

	// Nil config returns default
	var nilCfg *Config
	if got := nilCfg.GetTopLevelBool("warningsAsErrors", true); got != true {
		t.Errorf("expected default true from nil config, got %v", got)
	}
}

func TestData(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("style", "MagicNumber", "active", false)

	data := cfg.Data()
	if data == nil {
		t.Fatal("expected non-nil data map")
	}

	// Verify the data map contains what we set
	style, ok := data["style"].(map[string]interface{})
	if !ok {
		t.Fatal("expected style section in data")
	}
	mn, ok := style["MagicNumber"].(map[string]interface{})
	if !ok {
		t.Fatal("expected MagicNumber in style")
	}
	if mn["active"] != false {
		t.Errorf("expected active=false, got %v", mn["active"])
	}
}

func TestDataNilConfig(t *testing.T) {
	var cfg *Config
	if cfg.Data() != nil {
		t.Error("expected nil data from nil config")
	}
}

func TestDataEmptyConfig(t *testing.T) {
	cfg := NewConfig()
	data := cfg.Data()
	if data == nil {
		t.Fatal("expected non-nil (empty) map")
	}
	if len(data) != 0 {
		t.Errorf("expected empty map, got %d entries", len(data))
	}
}

func TestLoadEditorConfig(t *testing.T) {
	dir := t.TempDir()
	ecPath := filepath.Join(dir, ".editorconfig")
	os.WriteFile(ecPath, []byte(`
root = true

[*.kt]
max_line_length = 100
indent_size = 4
indent_style = space
tab_width = 8
charset = utf-8
insert_final_newline = true
trim_trailing_whitespace = true
`), 0644)

	ec := LoadEditorConfig(dir)

	if ec.MaxLineLength != 100 {
		t.Errorf("expected MaxLineLength=100, got %d", ec.MaxLineLength)
	}
	if ec.IndentSize != 4 {
		t.Errorf("expected IndentSize=4, got %d", ec.IndentSize)
	}
	if ec.IndentStyle != "space" {
		t.Errorf("expected IndentStyle='space', got %q", ec.IndentStyle)
	}
	if ec.Charset != "utf-8" {
		t.Errorf("expected Charset='utf-8', got %q", ec.Charset)
	}
	if ec.TabWidth != 8 {
		t.Errorf("expected TabWidth=8, got %d", ec.TabWidth)
	}
	if ec.InsertFinalNewline == nil || *ec.InsertFinalNewline != true {
		t.Errorf("expected InsertFinalNewline=true, got %v", ec.InsertFinalNewline)
	}
	if ec.TrimTrailingWhitespace == nil || *ec.TrimTrailingWhitespace != true {
		t.Errorf("expected TrimTrailingWhitespace=true, got %v", ec.TrimTrailingWhitespace)
	}
}

func TestLoadEditorConfigMaxLineLengthOff(t *testing.T) {
	dir := t.TempDir()
	ecPath := filepath.Join(dir, ".editorconfig")
	os.WriteFile(ecPath, []byte(`
root = true

[*]
max_line_length = off
`), 0644)

	ec := LoadEditorConfig(dir)
	if ec.MaxLineLength != -1 {
		t.Errorf("expected MaxLineLength=-1 for 'off', got %d", ec.MaxLineLength)
	}
}

func TestLoadEditorConfigNoFile(t *testing.T) {
	dir := t.TempDir()
	ec := LoadEditorConfig(dir)
	// Should return an empty EditorConfig with zero values
	if ec.MaxLineLength != 0 || ec.IndentSize != 0 {
		t.Errorf("expected zero values for missing editorconfig, got max=%d indent=%d",
			ec.MaxLineLength, ec.IndentSize)
	}
}

func TestLoadEditorConfigSkipsVendorProps(t *testing.T) {
	dir := t.TempDir()
	ecPath := filepath.Join(dir, ".editorconfig")
	os.WriteFile(ecPath, []byte(`
root = true

[*.kt]
max_line_length = 100
ktlint_standard_no-wildcard-imports = disabled
ij_kotlin_code_style_defaults = KOTLIN_OFFICIAL
ktfmt_style = google
`), 0644)

	ec := LoadEditorConfig(dir)
	if ec.MaxLineLength != 100 {
		t.Errorf("expected MaxLineLength=100, got %d", ec.MaxLineLength)
	}
	// Vendor props should be silently ignored (no error, no side effects)
}

func TestApplyToConfig(t *testing.T) {
	cfg := NewConfig()

	ec := &EditorConfig{
		MaxLineLength: 100,
		IndentStyle:   "tab",
	}
	falseVal := false
	ec.InsertFinalNewline = &falseVal
	ec.TrimTrailingWhitespace = &falseVal

	ec.ApplyToConfig(cfg)

	// max_line_length -> MaxLineLength rule
	if got := cfg.GetInt("style", "MaxLineLength", "maxLineLength", 0); got != 100 {
		t.Errorf("expected maxLineLength=100, got %d", got)
	}

	// insert_final_newline=false -> NewLineAtEndOfFile disabled
	active := cfg.IsRuleActive("style", "NewLineAtEndOfFile")
	if active == nil || *active != false {
		t.Errorf("expected NewLineAtEndOfFile active=false, got %v", active)
	}

	// trim_trailing_whitespace=false -> TrailingWhitespace disabled
	active = cfg.IsRuleActive("style", "TrailingWhitespace")
	if active == nil || *active != false {
		t.Errorf("expected TrailingWhitespace active=false, got %v", active)
	}

	// indent_style=tab -> NoTabs disabled
	active = cfg.IsRuleActive("style", "NoTabs")
	if active == nil || *active != false {
		t.Errorf("expected NoTabs active=false, got %v", active)
	}
}

func TestApplyToConfig_NoTabsIndentSize(t *testing.T) {
	t.Run("indent_size sets NoTabs.indentSize", func(t *testing.T) {
		cfg := NewConfig()
		ec := &EditorConfig{IndentSize: 2}
		ec.ApplyToConfig(cfg)
		if got := cfg.GetInt("style", "NoTabs", "indentSize", 0); got != 2 {
			t.Errorf("expected NoTabs.indentSize=2, got %d", got)
		}
	})
	t.Run("tab_width fallback when indent_size unset", func(t *testing.T) {
		cfg := NewConfig()
		ec := &EditorConfig{TabWidth: 8}
		ec.ApplyToConfig(cfg)
		if got := cfg.GetInt("style", "NoTabs", "indentSize", 0); got != 8 {
			t.Errorf("expected NoTabs.indentSize=8 from tab_width, got %d", got)
		}
	})
	t.Run("no editorconfig values leaves indentSize unset", func(t *testing.T) {
		cfg := NewConfig()
		ec := &EditorConfig{}
		ec.ApplyToConfig(cfg)
		if got := cfg.GetInt("style", "NoTabs", "indentSize", -1); got != -1 {
			t.Errorf("expected NoTabs.indentSize unset (default -1), got %d", got)
		}
	})
}

func TestApplyToConfigMaxLineLengthOff(t *testing.T) {
	cfg := NewConfig()
	ec := &EditorConfig{MaxLineLength: -1}
	ec.ApplyToConfig(cfg)

	active := cfg.IsRuleActive("style", "MaxLineLength")
	if active == nil || *active != false {
		t.Errorf("expected MaxLineLength disabled when max_line_length=off, got %v", active)
	}
}

func TestApplyToConfigNilSafety(t *testing.T) {
	// Nil editorconfig
	var ec *EditorConfig
	cfg := NewConfig()
	ec.ApplyToConfig(cfg) // should not panic

	// Nil config
	ec2 := &EditorConfig{MaxLineLength: 100}
	ec2.ApplyToConfig(nil) // should not panic
}

func TestLoadConfigInvalidTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "krit.yml")
	os.WriteFile(path, []byte("style:\n  MagicNumber:\n    active: true\n    threshold: 42\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.GetBool("style", "MagicNumber", "active", false) != true {
		t.Error("expected active=true")
	}
}

func TestLookupValue(t *testing.T) {
	data := map[string]interface{}{
		"top":   "value",
		"deep":  map[string]interface{}{"a": map[string]interface{}{"b": 42}},
		"mixed": map[string]interface{}{"leaf": []interface{}{"x", "y"}},
	}

	cases := []struct {
		name    string
		path    []string
		want    interface{}
		wantOk  bool
		isSlice bool
	}{
		{"top-level hit", []string{"top"}, "value", true, false},
		{"deep hit", []string{"deep", "a", "b"}, 42, true, false},
		{"slice leaf", []string{"mixed", "leaf"}, nil, true, true},
		{"missing top", []string{"nope"}, nil, false, false},
		{"missing mid", []string{"deep", "nope", "b"}, nil, false, false},
		{"missing leaf", []string{"deep", "a", "nope"}, nil, false, false},
		{"path through non-map", []string{"top", "more"}, nil, false, false},
		{"empty path", nil, nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := lookupValue(data, tc.path...)
			if ok != tc.wantOk {
				t.Fatalf("ok = %v, want %v (got = %v)", ok, tc.wantOk, got)
			}
			if !ok {
				return
			}
			if tc.isSlice {
				if _, isSlice := got.([]interface{}); !isSlice {
					t.Errorf("expected []interface{}, got %T", got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("got = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLookupValueNilData(t *testing.T) {
	if _, ok := lookupValue(nil, "any"); ok {
		t.Error("expected (nil, false) on nil data")
	}
}

func TestLookupMap(t *testing.T) {
	data := map[string]interface{}{
		"section": map[string]interface{}{"key": "val"},
		"scalar":  "not-a-map",
	}
	m, ok := lookupMap(data, "section")
	if !ok {
		t.Fatal("expected hit on section")
	}
	if m["key"] != "val" {
		t.Errorf("expected key=val, got %v", m["key"])
	}
	if _, ok := lookupMap(data, "scalar"); ok {
		t.Error("expected miss on scalar leaf")
	}
	if _, ok := lookupMap(data, "missing"); ok {
		t.Error("expected miss on missing key")
	}
}

func TestNewConfigFromData(t *testing.T) {
	cfg := NewConfigFromData(map[string]interface{}{
		"style": map[string]interface{}{
			"MagicNumber": map[string]interface{}{
				"active":    true,
				"threshold": 7,
			},
		},
	})
	if active := cfg.IsRuleActive("style", "MagicNumber"); active == nil || !*active {
		t.Errorf("expected active=true, got %v", active)
	}
	if got := cfg.GetInt("style", "MagicNumber", "threshold", 0); got != 7 {
		t.Errorf("expected threshold=7, got %d", got)
	}
	if got := cfg.GetInt("style", "MagicNumber", "missing", 99); got != 99 {
		t.Errorf("expected default 99 on missing key, got %d", got)
	}
}

func TestNewConfigFromDataNil(t *testing.T) {
	cfg := NewConfigFromData(nil)
	if cfg == nil {
		t.Fatal("expected non-nil config for nil data")
	}
	if cfg.IsRuleActive("x", "y") != nil {
		t.Error("expected nil for empty-config lookup")
	}
}

func TestTypedSections(t *testing.T) {
	cfg := NewConfigFromData(map[string]interface{}{
		"analysis": map[string]interface{}{"depth": "deep"},
		"parseCache": map[string]interface{}{
			"maxSizeMB": 512,
		},
		"lsp": map[string]interface{}{
			"classpath": []interface{}{"/a.jar", "/b.jar"},
		},
		"oracle": map[string]interface{}{
			"classpath": []interface{}{"/c.jar"},
		},
	})

	if got := cfg.Analysis().Depth; got != "deep" {
		t.Errorf("Analysis().Depth = %q, want %q", got, "deep")
	}
	if got := cfg.ParseCache().MaxSizeMB; got != 512 {
		t.Errorf("ParseCache().MaxSizeMB = %d, want %d", got, 512)
	}
	lspCP := cfg.LSP().Classpath
	if len(lspCP) != 2 || lspCP[0] != "/a.jar" || lspCP[1] != "/b.jar" {
		t.Errorf("LSP().Classpath = %v, want [/a.jar /b.jar]", lspCP)
	}
	oracleCP := cfg.Oracle().Classpath
	if len(oracleCP) != 1 || oracleCP[0] != "/c.jar" {
		t.Errorf("Oracle().Classpath = %v, want [/c.jar]", oracleCP)
	}
}

func TestTypedSectionsEmpty(t *testing.T) {
	cfg := NewConfigFromData(nil)
	if got := cfg.Analysis().Depth; got != "" {
		t.Errorf("expected empty Depth on empty config, got %q", got)
	}
	if got := cfg.ParseCache().MaxSizeMB; got != 0 {
		t.Errorf("expected MaxSizeMB=0 on empty config, got %d", got)
	}
	if got := cfg.LSP().Classpath; got != nil {
		t.Errorf("expected nil LSP classpath on empty config, got %v", got)
	}
	if got := cfg.Oracle().Classpath; got != nil {
		t.Errorf("expected nil Oracle classpath on empty config, got %v", got)
	}
}

func TestTypedSectionsNilConfig(t *testing.T) {
	var cfg *Config
	// Each typed accessor must tolerate a nil receiver and return the
	// zero value of its struct (matching the underlying GetTopLevel*
	// contract).
	if got := cfg.Analysis().Depth; got != "" {
		t.Errorf("nil cfg Analysis().Depth = %q, want empty", got)
	}
	if got := cfg.ParseCache().MaxSizeMB; got != 0 {
		t.Errorf("nil cfg ParseCache().MaxSizeMB = %d, want 0", got)
	}
	if got := cfg.LSP().Classpath; got != nil {
		t.Errorf("nil cfg LSP().Classpath = %v, want nil", got)
	}
	if got := cfg.Oracle().Classpath; got != nil {
		t.Errorf("nil cfg Oracle().Classpath = %v, want nil", got)
	}
}
