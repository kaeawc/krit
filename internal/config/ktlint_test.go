package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// ktor's .editorconfig disables ktlint's no-wildcard-imports and lists the
// packages IntelliJ imports on demand: WildcardImport stays quiet there
// (5,086 corpus findings before).
func TestKtlintEditorConfigLayer(t *testing.T) {
	installFakeView(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLintTestFile(t, root, ".editorconfig", `root = true

[*]
indent_style = space

[*.{kt,kts}]
ij_kotlin_packages_to_use_import_on_demand = java.util.*, io.ktor.**
ktlint_standard_no-wildcard-imports = disabled
ktlint_standard_filename = disabled
ktlint_standard_trailing-comma-on-call-site = disabled
ktlint_standard_max-line-length = enabled

[*.md]
ktlint_standard_function-naming = disabled
`)
	layer, info := loadKtlintLayer([]string{root})
	if info == nil || info.Source != filepath.Join(root, ".editorconfig") {
		t.Fatalf("info = %+v", info)
	}
	cfg := NewConfigFromData(layer)
	if a := cfg.IsRuleActive("style", "WildcardImport"); a == nil || *a {
		t.Error("WildcardImport not turned off")
	}
	if a := cfg.IsRuleActive("naming", "FunctionNaming"); a != nil {
		t.Error("a [*.md] section applied to Kotlin")
	}
	if got := cfg.GetStringList("style", "WildcardImport", "excludeImports"); !slices.Equal(got, []string{"java.util.*", "io.ktor.**"}) {
		t.Errorf("excludeImports = %v", got)
	}
	if a := cfg.IsRuleActive("style", "MaxLineLength"); a != nil {
		t.Error("an enabled ktlint rule changed MaxLineLength")
	}
}

// The ktlint layer sits under the detekt config and krit.yml, keeps krit's
// shipped WildcardImport exclusions, and can be turned off.
func TestKtlintEditorConfigPrecedenceAndOptOut(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLintTestFile(t, root, ".editorconfig", "root = true\n[*.kt]\nktlint_standard_no-wildcard-imports = disabled\nktlint_standard_function-naming = disabled\nij_kotlin_packages_to_use_import_on_demand = io.ktor.**\n")
	writeLintTestFile(t, root, "krit.yml", "naming:\n  FunctionNaming:\n    active: true\n")
	cfg, err := LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if a := cfg.IsRuleActive("style", "WildcardImport"); a == nil || *a {
		t.Error("WildcardImport not turned off by .editorconfig")
	}
	if a := cfg.IsRuleActive("naming", "FunctionNaming"); a == nil || !*a {
		t.Error("krit.yml did not win over .editorconfig")
	}
	got := cfg.GetStringList("style", "WildcardImport", "excludeImports")
	for _, want := range []string{"java.util.*", "platform.**", "io.ktor.**"} {
		if !slices.Contains(got, want) {
			t.Errorf("excludeImports = %v, missing %s", got, want)
		}
	}

	writeLintTestFile(t, root, "krit.yml", "ktlintEditorConfig:\n  enabled: false\n")
	cfg, err = LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if a := cfg.IsRuleActive("style", "WildcardImport"); a != nil && !*a {
		t.Error("ktlintEditorConfig.enabled: false still applied .editorconfig")
	}
}

// element-x-android sets max_line_length = 160 and got 1,827 MaxLineLength
// findings at krit's default of 120; moshi sets 100.
func TestKtlintEditorConfigMaxLineLength(t *testing.T) {
	installFakeView(t)
	for _, tc := range []struct {
		value  string
		limit  int
		active *bool
	}{
		{"160", 160, nil},
		{"off", 0, func() *bool { b := false; return &b }()},
		{"unset", 0, nil},
	} {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		content := "root = true\n[*.{kt,kts}]\n"
		if tc.value != "unset" {
			content += "max_line_length = " + tc.value + "\n"
		}
		writeLintTestFile(t, root, ".editorconfig", content)
		layer, _ := loadKtlintLayer([]string{root})
		cfg := NewConfigFromData(layer)
		if got := cfg.GetInt("style", "MaxLineLength", "maxLineLength", 0); got != tc.limit {
			t.Errorf("%s: maxLineLength = %d, want %d", tc.value, got, tc.limit)
		}
		got := cfg.IsRuleActive("style", "MaxLineLength")
		if (got == nil) != (tc.active == nil) || got != nil && *got != *tc.active {
			t.Errorf("%s: active = %v, want %v", tc.value, got, tc.active)
		}
	}
}
