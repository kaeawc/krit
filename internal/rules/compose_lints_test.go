package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
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
