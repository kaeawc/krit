package rules_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestComposeLintsNamingLintIDsSuppressTheirDirection checks that each compose-lints naming ID silences only the
// ComposableNaming findings of its casing direction, as @SuppressLint and @Suppress do in Lint.
func TestComposeLintsNamingLintIDsSuppressTheirDirection(t *testing.T) {
	const src = `
package test

import android.annotation.SuppressLint
import androidx.compose.runtime.Composable

%s
@Composable
fun myButton() {}

%s
@Composable
fun Title(): String {
    return "x"
}
`
	run := func(unitAnn, valueAnn string) int {
		code := strings.Replace(strings.Replace(src, "%s", unitAnn, 1), "%s", valueAnn, 1)
		return len(runRuleByNameOnFile(t, "ComposableNaming", parseInline(t, code)))
	}
	if got := run("", ""); got != 2 {
		t.Fatalf("expected 2 findings without suppression, got %d", got)
	}
	cases := []struct {
		unitAnn, valueAnn string
		want              int
	}{
		{`@SuppressLint("ComposeNamingUppercase")`, "", 1},
		{"", `@Suppress("ComposeNamingLowercase")`, 1},
		{`@SuppressLint("ComposeNamingLowercase")`, `@Suppress("ComposeNamingUppercase")`, 2},
		{`@SuppressLint("ComposeNamingUppercase")`, `@SuppressLint("ComposeNamingLowercase")`, 0},
		{`@Suppress("ComposableNaming")`, "", 1},
	}
	for _, c := range cases {
		if got := run(c.unitAnn, c.valueAnn); got != c.want {
			t.Errorf("unit %q, value %q: got %d findings, want %d", c.unitAnn, c.valueAnn, got, c.want)
		}
	}
}

// TestComposeLintsNamingIDsDoNotDisableComposableNaming checks that the compose-lints IDs are not Lint aliases that
// a lint.xml ignore of one of them would extend to the whole AndroidX rule.
func TestComposeLintsNamingIDsDoNotDisableComposableNaming(t *testing.T) {
	for _, r := range api.Registry {
		if r.ID != "ComposableNaming" {
			continue
		}
		for _, alias := range append(rules.LintSuppressionAliases(r), r.Aliases...) {
			if alias == "ComposeNamingUppercase" || alias == "ComposeNamingLowercase" {
				t.Errorf("ComposableNaming must not alias %s", alias)
			}
		}
		return
	}
	t.Fatal("ComposableNaming not registered")
}

func TestComposeCompositionLocalUsageSuppressLint(t *testing.T) {
	const src = `
package test

import android.annotation.SuppressLint
import androidx.compose.runtime.compositionLocalOf

%s
val LocalThing = compositionLocalOf { 1 }
`
	if got := runRuleByNameOnFile(t, "ComposeCompositionLocalUsage", parseInline(t, strings.Replace(src, "%s", "", 1))); len(got) != 1 {
		t.Fatalf("expected 1 finding without suppression, got %d", len(got))
	}
	got := runRuleByNameOnFile(t, "ComposeCompositionLocalUsage",
		parseInline(t, strings.Replace(src, "%s", `@SuppressLint("ComposeCompositionLocalUsage")`, 1)))
	if len(got) != 0 {
		t.Errorf("@SuppressLint must suppress ComposeCompositionLocalUsage, got %d findings", len(got))
	}
}

// TestComposeCompositionLocalRulesNeedRuntimeFactory checks the lexical and shadowing negatives: a factory named in
// a string or comment, a qualified non-runtime callee, and a member function shadowing the import.
func TestComposeCompositionLocalRulesNeedRuntimeFactory(t *testing.T) {
	const src = `
package test

import androidx.compose.runtime.compositionLocalOf

// val LocalComment = compositionLocalOf { 1 }
val text = "val LocalString = compositionLocalOf { 1 }"
val LocalOther = other.compositionLocalOf { 1 }

class Shadow {
    fun <T> compositionLocalOf(f: () -> T): T = f()
    val LocalShadowed = compositionLocalOf { 2 }
    val LocalShadowedGetter get() = compositionLocalOf { 3 }
}
`
	for _, rule := range []string{"ComposeCompositionLocalUsage", "ComposeCompositionLocalGetter"} {
		if got := runRuleByNameOnFile(t, rule, parseInline(t, src)); len(got) != 0 {
			t.Errorf("%s: expected no findings, got %v", rule, got)
		}
	}
}

// TestComposeLintsCrossFileFixtures runs the project-scope ports over their fixtures, which TestPositiveFixtures
// and TestNegativeFixtures skip.
func TestComposeLintsCrossFileFixtures(t *testing.T) {
	for _, rule := range []string{"ComposePreviewPublic", "ComposePreviewNaming", "ComposeM2Api",
		"ComposeModifierMissing", "ComposeParameterOrder", "ComposeViewModelForwarding",
		"ComposeUnstableCollections"} {
		r := findComposeLintsRule(t, rule)
		pos := parseFixture(t, "positive", rule)
		if got := runCrossFileRuleOnFiles(r, pos); len(got) == 0 {
			t.Errorf("%s: expected findings on the positive fixture", rule)
		}
		neg := parseFixture(t, "negative", rule)
		if got := runCrossFileRuleOnFiles(r, neg); len(got) != 0 {
			t.Errorf("%s: expected no findings on the negative fixture, got %v", rule, got)
		}
	}
}

func TestComposePreviewPublicSuppressLint(t *testing.T) {
	const src = `
package test

import android.annotation.SuppressLint
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

%s
@Preview
@Composable
fun ScreenPreview() {}
`
	r := findComposeLintsRule(t, "ComposePreviewPublic")
	if got := runCrossFileRuleOnFiles(r, parseInline(t, strings.Replace(src, "%s", "", 1))); len(got) != 1 {
		t.Fatalf("expected 1 finding without suppression, got %d", len(got))
	}
	if got := runCrossFileRuleOnFiles(r, parseInline(t, strings.Replace(src, "%s", `@SuppressLint("ComposePreviewPublic")`, 1))); len(got) != 0 {
		t.Errorf("@SuppressLint must suppress ComposePreviewPublic, got %d findings", len(got))
	}
}

// TestComposePreviewPublicCrossFileMultipreview checks that a multipreview declared in another file counts, and that
// a same-named annotation in another package does not.
func TestComposePreviewPublicCrossFileMultipreview(t *testing.T) {
	annotations := parseInlineWithName(t, "Previews.kt", `
package com.example.tooling

import androidx.compose.ui.tooling.preview.Preview

@Preview
annotation class DevicePreviews
`)
	other := parseInlineWithName(t, "Other.kt", `
package com.example.other

annotation class DevicePreviews
`)
	screen := parseInlineWithName(t, "Screen.kt", `
package com.example.ui

import androidx.compose.runtime.Composable
import com.example.tooling.DevicePreviews

@DevicePreviews
@Composable
fun ScreenPreview() {}
`)
	lookalike := parseInlineWithName(t, "Lookalike.kt", `
package com.example.ui2

import androidx.compose.runtime.Composable
import com.example.other.DevicePreviews

@DevicePreviews
@Composable
fun NotAPreview() {}
`)
	r := findComposeLintsRule(t, "ComposePreviewPublic")
	got := runCrossFileRuleOnFiles(r, annotations, other, screen, lookalike)
	if len(got) != 1 || !strings.HasSuffix(got[0].File, "Screen.kt") {
		t.Errorf("expected exactly one finding in Screen.kt, got %v", got)
	}
}

func findComposeLintsRule(t *testing.T, id string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %q not registered", id)
	return nil
}

func parseFixture(t *testing.T, kind, rule string) *scanner.File {
	t.Helper()
	file, err := scanner.ParseFile(context.Background(), filepath.Join(fixtureRoot(t), kind, "compose", rule+".kt"))
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// TestComposeModifierWithoutDefaultFixSpelling pins the autofix's default: the parameter type as written when it
// resolves to Modifier or GlanceModifier, and no fix for a lookalike type or a vararg parameter (the finding stays,
// as in Lint, which matches the type's text).
func TestComposeModifierWithoutDefaultFixSpelling(t *testing.T) {
	for _, tc := range []struct {
		name, imports, param, want string
	}{
		{"compose", "import androidx.compose.ui.Modifier", "modifier: Modifier", " = Modifier"},
		{"nullable", "import androidx.compose.ui.Modifier", "modifier: Modifier?", " = Modifier"},
		{"glance", "import androidx.glance.GlanceModifier", "modifier: GlanceModifier", " = GlanceModifier"},
		{"qualified", "", "modifier: androidx.compose.ui.Modifier", " = androidx.compose.ui.Modifier"},
		{"alias", "import androidx.compose.ui.Modifier as M", "m: M", " = M"},
		{"vararg", "import androidx.compose.ui.Modifier", "vararg modifier: Modifier", ""},
		{"reflect lookalike", "import java.lang.reflect.Modifier", "modifier: Modifier", ""},
		{"same-file lookalike", "class GlanceModifier", "modifier: GlanceModifier", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := assertFlags(t, "ComposeModifierWithoutDefault", "package test\n\nimport androidx.compose.runtime.Composable\n"+
				tc.imports+"\n\n@Composable\nfun Widget("+tc.param+") {}\n")
			got := ""
			if fix := findings[0].Fix; fix != nil {
				got = fix.Replacement
			}
			if got != tc.want {
				t.Errorf("fix replacement = %q, want %q", got, tc.want)
			}
		})
	}
}
