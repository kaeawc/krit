package rules_test

import (
	"strings"
	"testing"
)

// TestComposeLintsNamingLintIDsSuppressComposableNaming checks that compose-lints' issue IDs silence the krit rule
// they alias, as @SuppressLint and @Suppress do in Lint.
func TestComposeLintsNamingLintIDsSuppressComposableNaming(t *testing.T) {
	const src = `
package test

import android.annotation.SuppressLint
import androidx.compose.runtime.Composable

%s
@Composable
fun myButton() {}
`
	if got := runRuleByNameOnFile(t, "ComposableNaming", parseInline(t, strings.Replace(src, "%s", "", 1))); len(got) != 1 {
		t.Fatalf("expected 1 finding without suppression, got %d", len(got))
	}
	for _, ann := range []string{`@SuppressLint("ComposeNamingUppercase")`, `@Suppress("ComposeNamingLowercase")`} {
		if got := runRuleByNameOnFile(t, "ComposableNaming", parseInline(t, strings.Replace(src, "%s", ann, 1))); len(got) != 0 {
			t.Errorf("%s must suppress ComposableNaming, got %d findings", ann, len(got))
		}
	}
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
