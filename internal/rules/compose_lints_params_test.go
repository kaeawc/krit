package rules_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression tests for #1948, the follow-ups to the parameter-type and ViewModel ports. Each case was checked
// against the compose-lints 1.6.1 detector sources.

// TestComposeViewModelInjectionConfiguredFactoryWithoutViewModelText checks that a configured factory is reported
// in a file whose text never says "ViewModel": the file gate must not hide configured factories.
func TestComposeViewModelInjectionConfiguredFactoryWithoutViewModelText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "krit-options.txt"),
		[]byte("ComposeViewModelInjection.viewModelFactories = rememberPresenter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyComposeLintsOptions(t, dir)
	assertFlags(t, "ComposeViewModelInjection", `
package test

import androidx.compose.runtime.Composable

class HomePresenter

@Composable
fun Home() {
    val presenter = rememberPresenter<HomePresenter>()
}
`)
}

// TestComposeViewModelForwardingLocalFunctionParameterShadows checks that a local function's own parameter
// shadows the outer composable's ViewModel parameter of the same name.
func TestComposeViewModelForwardingLocalFunctionParameterShadows(t *testing.T) {
	r := findComposeLintsRule(t, "ComposeViewModelForwarding")
	file := parseInline(t, `
package test

import androidx.compose.runtime.Composable

class MyViewModel

@Composable
fun Child2(vm: MyViewModel) {}

@Composable
fun Screen(vm: MyViewModel) {
    @Composable
    fun inner(vm: MyViewModel) {
        Child2(vm)
    }
}
`)
	if got := runCrossFileRuleOnFiles(r, file); len(got) != 0 {
		t.Errorf("a local function's parameter shadows the outer ViewModel, got %v", got)
	}
	// Pin: without the local rebinding, the outer parameter is still forwarded.
	file = parseInline(t, `
package test

import androidx.compose.runtime.Composable

class MyViewModel

@Composable
fun Child2(vm: MyViewModel) {}

@Composable
fun Screen(vm: MyViewModel) {
    @Composable
    fun inner(other: Int) {
        Child2(vm)
    }
}
`)
	if got := runCrossFileRuleOnFiles(r, file); len(got) != 1 {
		t.Errorf("the outer ViewModel forwarded from a local function must be reported once, got %v", got)
	}
}

// TestComposeViewModelForwardingKotlinConstructors checks that kotlin and Android constructors that the project
// does not declare (Pair, Triple, Intent, WeakReference) are not taken for composables.
func TestComposeViewModelForwardingKotlinConstructors(t *testing.T) {
	r := findComposeLintsRule(t, "ComposeViewModelForwarding")
	file := parseInline(t, `
package test

import android.content.Intent
import androidx.compose.runtime.Composable
import java.lang.ref.WeakReference

class MyViewModel

@Composable
fun Screen(vm: MyViewModel) {
    val p = Pair(vm, 1)
    val t = Triple(vm, 1, 2)
    val r = WeakReference(vm)
    val i = Intent(vm)
}
`)
	if got := runCrossFileRuleOnFiles(r, file); len(got) != 0 {
		t.Errorf("constructor calls are not composables, got %v", got)
	}
}

// TestComposeViewModelForwardingKDocIsNotTheAnnotation checks that "Composable" in a KDoc comment is not read as
// the @Composable annotation of the callee's declaration.
func TestComposeViewModelForwardingKDocIsNotTheAnnotation(t *testing.T) {
	r := findComposeLintsRule(t, "ComposeViewModelForwarding")
	file := parseInline(t, `
package test

import androidx.compose.runtime.Composable

class MyViewModel

/** Not a Composable. */
fun Foo(vm: MyViewModel) {}

@Deprecated("Use the Composable overload")
fun Bar(vm: MyViewModel) {}

@Suppress("unused")
/** Not a Composable either. */
fun Baz(vm: MyViewModel) {}

@Composable
fun Screen(vm: MyViewModel) {
    Foo(vm)
    Bar(vm)
    Baz(vm)
}
`)
	if got := runCrossFileRuleOnFiles(r, file); len(got) != 0 {
		t.Errorf("a comment or string mentioning Composable is not @Composable, got %v", got)
	}
}

// TestComposeViewModelForwardingCalleeResolution pins the cases around the platform-class check: a project
// composable, a library composable and an undeclared, unimported name stay reported; a project class does not.
func TestComposeViewModelForwardingCalleeResolution(t *testing.T) {
	r := findComposeLintsRule(t, "ComposeViewModelForwarding")
	file := parseInline(t, `
package test

import androidx.compose.runtime.Composable
import com.example.ui.Card

class MyViewModel
class Holder(vm: MyViewModel)

@Composable
fun Pair(vm: MyViewModel) {}

@Composable
fun Screen(vm: MyViewModel) {
    Pair(vm)
    Card(vm)
    Unknown(vm)
    Holder(vm)
}
`)
	if got := runCrossFileRuleOnFiles(r, file); len(got) != 3 {
		t.Errorf("want 3 findings (project composable Pair, imported Card, unresolved Unknown), got %v", got)
	}
}

// TestComposeLintsVarargParameters checks that a vararg parameter's type is its array type, as UAST sees it: only
// MutableParameters' text match on a written Kotlin mutable collection still reports it.
func TestComposeLintsVarargParameters(t *testing.T) {
	const header = "package test\n\nimport androidx.compose.runtime.Composable\n\n@Composable\nfun Widget("
	for _, tc := range []struct {
		rule, param string
		want        int
	}{
		{"ComposeMutableParameters", "vararg items: ArrayList<String>", 0},
		{"ComposeMutableParameters", "vararg items: MutableState<String>", 0},
		{"ComposeMutableParameters", "vararg items: MutableList<String>", 1},
		{"ComposeMutableParameters", "vararg items: MutableMap<String, Int>", 1},
		{"ComposeMutableParameters", "items: ArrayList<String>", 1},
		{"ComposeUnstableCollections", "vararg items: List<String>", 0},
		{"ComposeUnstableCollections", "vararg items: MutableList<String>", 0},
		{"ComposeUnstableCollections", "items: List<String>", 1},
	} {
		t.Run(tc.rule+"/"+tc.param, func(t *testing.T) {
			src := header + tc.param + ") {}\n"
			r := findComposeLintsRule(t, tc.rule)
			var n int
			if tc.rule == "ComposeUnstableCollections" {
				n = len(runCrossFileRuleOnFiles(r, parseInline(t, src)))
			} else {
				n = len(findingsFor(t, r, inlineKotlin(t, src)))
			}
			if n != tc.want {
				t.Errorf("%d findings, want %d", n, tc.want)
			}
		})
	}
}
