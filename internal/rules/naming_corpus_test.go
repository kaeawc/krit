package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// Kotlin's coding conventions recommend omitting the common root package
// from the directory layout. kotlinx.coroutines, ktor and
// kotlinx.serialization do (about 1,240 findings across the three before
// this was accepted).
func TestInvalidPackageDeclaration_AcceptsOmittedCommonRoot(t *testing.T) {
	for _, tc := range []struct {
		path, pkg string
		want      int
	}{
		{"kotlinx-coroutines-core/common/src/flow/internal/SafeCollector.kt", "kotlinx.coroutines.flow.internal", 0},
		{"kotlinx-coroutines-core/common/test/WithTimeoutTest.kt", "kotlinx.coroutines", 0},
		{"kotlinx-coroutines-core/jvm/test/scheduling/LivenessTest.kt", "kotlinx.coroutines.scheduling", 0},
		{"ktor-client-tracing/common/src/TracingWrapper.kt", "io.ktor.client.plugin.tracing", 0},
		{"core/commonMain/src/kotlinx/serialization/Foo.kt", "kotlinx.serialization", 0},
		{"lib/src/jvmMain/kotlin/feature/Foo.kt", "com.example.feature", 0},
		{"app/src/main/kotlin/com/example/feature/Foo.kt", "com.example.feature", 0},

		// Still misplaced: the trailing directories are not the package's
		// trailing segments, or they sit below a package directory rather
		// than a source root.
		{"app/src/main/kotlin/com/example/other/Foo.kt", "com.example.feature", 1},
		{"app/src/main/kotlin/com/example/Foo.kt", "com.example.feature", 1},
		{"app/src/main/kotlin/feature/sub/Foo.kt", "com.example.feature", 1},
		{"tests/fixtures/positive/naming/Foo.kt", "com.wrong.path", 1},
	} {
		t.Run(tc.path, func(t *testing.T) {
			findings := runRuleByNameOnPath(t, "InvalidPackageDeclaration", tc.path, "package "+tc.pkg+"\n\nclass Foo\n")
			if len(findings) != tc.want {
				t.Errorf("findings = %d, want %d", len(findings), tc.want)
			}
		})
	}
}

// An explicit rootPackage keeps the strict comparison: only that root may
// be omitted.
func TestInvalidPackageDeclaration_ConfiguredRootPackageStaysStrict(t *testing.T) {
	var rule *rules.InvalidPackageDeclarationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "InvalidPackageDeclaration" {
			rule = candidate.Implementation.(*rules.InvalidPackageDeclarationRule)
		}
	}
	original := rule.RootPackage
	defer func() { rule.RootPackage = original }()
	rule.RootPackage = "com.example"

	if got := len(runRuleByNameOnPath(t, "InvalidPackageDeclaration", "src/main/kotlin/feature/Foo.kt", "package com.example.feature\nclass Foo\n")); got != 0 {
		t.Errorf("configured root omitted: findings = %d, want 0", got)
	}
	if got := len(runRuleByNameOnPath(t, "InvalidPackageDeclaration", "common/src/flow/Foo.kt", "package kotlinx.coroutines.flow\nclass Foo\n")); got != 1 {
		t.Errorf("other root omitted with rootPackage set: findings = %d, want 1", got)
	}
}

// The shipped default config exempts composables, whose names follow
// Compose's PascalCase convention (1,866 findings on element-x-android and
// 534 on compose-samples before). Only the function's own annotations count.
func TestFunctionNaming_ShippedDefaultIgnoresComposables(t *testing.T) {
	cfg, err := config.LoadAndMerge("", filepath.Join("..", "..", "config", "default-krit.yml"))
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	var rule *rules.FunctionNamingRule
	for _, candidate := range api.Registry {
		if candidate.ID == "FunctionNaming" {
			rule = candidate.Implementation.(*rules.FunctionNamingRule)
		}
	}
	original := rule.IgnoreAnnotated
	defer func() { rule.IgnoreAnnotated = original }()
	api.ApplyConfig(rule, rule.Meta(), rules.NewConfigAdapter(cfg))

	for _, tc := range []struct {
		name, code string
		want       int
	}{
		{"composable", "@Composable\nfun SettingsButtons() {}", 0},
		{"composable after kdoc", "/** Docs. */\n@Composable\ninternal fun ListItemPreview() = Unit", 0},
		{"qualified preview composable", "@androidx.compose.ui.tooling.preview.Preview\n@androidx.compose.runtime.Composable\nprivate fun SearchToolbarPreview() {}", 0},
		{"not a composable", "fun SettingsButtons() {}", 1},
		{"composable parameter only", "fun Scaffold(content: @Composable () -> Unit) {}", 1},
		{"composable lambda in body", "fun Host() { val c: @Composable () -> Unit = {} }", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(runRuleByName(t, "FunctionNaming", "package test\n"+tc.code+"\n")); got != tc.want {
				t.Errorf("findings = %d, want %d", got, tc.want)
			}
		})
	}
}
