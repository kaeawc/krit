package rules_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// checkComposeLintsWants runs each rule over the sources (file name to Kotlin source) and requires its findings to
// sit exactly on the lines marked `// want <RuleID>`.
func checkComposeLintsWants(t *testing.T, sources map[string]string, ruleIDs ...string) {
	t.Helper()
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	files := make([]*scanner.File, 0, len(names))
	for _, name := range names {
		files = append(files, parseInlineWithName(t, name, sources[name]))
	}
	for _, id := range ruleIDs {
		var want []string
		for _, name := range names {
			for i, line := range strings.Split(sources[name], "\n") {
				if strings.Contains(line, "// want "+id) {
					want = append(want, fmt.Sprintf("%s:%d", name, i+1))
				}
			}
		}
		var got []string
		for _, f := range runCrossFileRuleOnFiles(findComposeLintsRule(t, id), files...) {
			got = append(got, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
		}
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: got findings at %v, want %v", id, got, want)
		}
	}
}

// TestComposeRedundantComposableLibraryTables checks composables whose JVM names carry inline-class mangling.
func TestComposeRedundantComposableLibraryTables(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Fab.kt": `package demo

import androidx.compose.foundation.text.input.TextFieldState
import androidx.compose.foundation.text.input.rememberTextFieldState
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.FloatingActionButtonDefaults
import androidx.compose.runtime.Composable

@Composable
fun rememberQueryState(): TextFieldState = rememberTextFieldState()

@Composable
fun fabElevation() = FloatingActionButtonDefaults.elevation()

@Composable
fun EmptyFab(onClick: () -> Unit) {
    FloatingActionButton(onClick = onClick, content = {})
}

@Composable // want ComposeRedundantComposable
fun NoComposition(onClick: () -> Unit) {
    onClick()
}
`}, "ComposeRedundantComposable")
}

// TestComposeRedundantComposableImplicitReceivers checks members reached through an extension receiver or an
// enclosing class: library scope members are composable, and a class extending an unscanned library type
// abstains on names krit cannot place.
func TestComposeRedundantComposableImplicitReceivers(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Scopes.kt": `package demo

import androidx.compose.animation.SharedTransitionScope
import androidx.compose.material3.ExposedDropdownMenuBoxScope
import androidx.compose.runtime.Composable
import com.lib.BaseScreen

@Composable
fun ExposedDropdownMenuBoxScope.Menu(expanded: Boolean) {
    ExposedDropdownMenu(expanded = expanded, onDismissRequest = {}) {}
}

@Composable
fun SharedTransitionScope.avatarState() = rememberSharedContentState("avatar")

class Home : BaseScreen() {
    @Composable
    fun Helper() {
        Body()
    }
}

class Plain {
    @Composable // want ComposeRedundantComposable
    fun Helper() {
        println("x")
    }
}

@Composable // want ComposeRedundantComposable
fun Logs(items: List<String>) {
    items.forEach { println(it) }
}
`}, "ComposeRedundantComposable")
}

// TestComposeRedundantComposableContractGetters checks that override and open getters, whose @Composable a
// supertype requires, are not reported.
func TestComposeRedundantComposableContractGetters(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Theme.kt": `package demo

import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

interface Themed {
    val color: Color
        @Composable get
}

class ThemedImpl : Themed {
    override val color: Color
        @Composable get() = Color.Red
}

open class OpenThemed {
    open val tint: Color
        @Composable get() = Color.Blue
}

class PlainThemed {
    val tint: Color
        @Composable get() = Color.Blue // want ComposeRedundantComposable
}
`}, "ComposeRedundantComposable")
}

// TestComposeReadOnlyComposableStableAcrossRuns checks that mixed scanned and unscanned star imports classify the
// same way every run: an unscanned package may declare a better overload, so krit abstains.
func TestComposeReadOnlyComposableStableAcrossRuns(t *testing.T) {
	for i := 0; i < 30; i++ {
		checkComposeLintsWants(t, map[string]string{"Label.kt": `package demo

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.*
import com.squareup.picasso.*

@Composable
fun label(): String = stringResource(1)
`}, "ComposeReadOnlyComposable")
	}
	checkComposeLintsWants(t, map[string]string{"Label.kt": `package demo

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.*

@Composable // want ComposeReadOnlyComposable
fun label(): String = stringResource(1)
`}, "ComposeReadOnlyComposable")
}

// TestComposeRedundantComposableProjectShadowing checks Kotlin's lookup order: a project function in another,
// unimported package does not shadow a star-imported composable, while one in the file's own package does.
func TestComposeRedundantComposableProjectShadowing(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{
		"Util.kt": `package com.ex.util

fun Icon(x: Int) = x
`,
		"Screen.kt": `package com.ex.ui

import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.vector.ImageVector

@Composable
fun WildcardIcon(v: ImageVector) {
    Icon(v, null)
}
`,
		"Local.kt": `package com.ex.local

import androidx.compose.material3.*
import androidx.compose.runtime.Composable

fun Icon(x: Int) = x

@Composable // want ComposeRedundantComposable
fun UsesLocal() {
    Icon(1)
}
`,
	}, "ComposeRedundantComposable")
}

// TestComposeRedundantComposableLookalikes checks lookalike annotations from other packages: a local @Composable
// is not a composable, and a local @ReadOnlyComposable does not make callers read-only.
func TestComposeRedundantComposableLookalikes(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{
		"Other.kt": `package com.other

annotation class Composable
annotation class ReadOnlyComposable
`,
		"Fake.kt": `package demo

import com.other.Composable

@Composable
fun NotReally() {
    println("x")
}
`,
		"Callers.kt": `package demo2

import androidx.compose.runtime.Composable
import com.other.ReadOnlyComposable

@Composable
@ReadOnlyComposable
fun fake(): Int = remember()

@Composable
fun remember(): Int = 1

@Composable
fun caller(): Int = fake()

@Composable
@androidx.compose.runtime.ReadOnlyComposable
fun real(): Int = 1

@Composable // want ComposeReadOnlyComposable
fun realCaller(): Int = real()
`,
	}, "ComposeReadOnlyComposable")
}

// TestComposeReadOnlyComposableLocalCurrent checks that `Local*.current` counts as a CompositionLocal read only
// when the name resolves to one.
func TestComposeReadOnlyComposableLocalCurrent(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Locals.kt": `package demo

import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf

object LocalSettings {
    val current = 1
}

val LocalThing = staticCompositionLocalOf { 1 }

@Composable // want ComposeRedundantComposable
fun settings() = LocalSettings.current

@Composable // want ComposeReadOnlyComposable
fun thing() = LocalThing.current
`}, "ComposeRedundantComposable", "ComposeReadOnlyComposable")
	checkComposeLintsWants(t, map[string]string{"Panel.kt": `package demo

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

object LocalPanel {
    val current: Int
        @Composable get() = remember { 1 }
}

@Composable
fun panel() = LocalPanel.current
`}, "ComposeRedundantComposable", "ComposeReadOnlyComposable")
}

// TestComposeRedundantComposableIgnoreAnnotated checks ignoreAnnotated against a same-package annotation and
// compose-lints' exemption for the Unit `operator fun invoke` of an ignored-annotated object.
func TestComposeRedundantComposableIgnoreAnnotated(t *testing.T) {
	r := findComposeLintsRule(t, "ComposeRedundantComposable")
	impl, ok := r.Implementation.(*rules.ComposeRedundantComposableRule)
	if !ok {
		t.Fatalf("unexpected implementation %T", r.Implementation)
	}
	saved := impl.IgnoreAnnotated
	impl.IgnoreAnnotated = []string{"demo.KeepComposable"}
	t.Cleanup(func() { impl.IgnoreAnnotated = saved })
	checkComposeLintsWants(t, map[string]string{"Kept.kt": `package demo

import androidx.compose.runtime.Composable

annotation class KeepComposable

@KeepComposable
@Composable
fun Kept() {
    println("x")
}

@KeepComposable
object Screen {
    @Composable
    operator fun invoke() {
        println("x")
    }
}

object Other {
    @Composable // want ComposeRedundantComposable
    operator fun invoke() {
        println("x")
    }
}

@Composable // want ComposeRedundantComposable
fun NotKept() {
    println("x")
}
`}, "ComposeRedundantComposable")
}

// TestComposeRedundantComposableValuesKritCannotProve checks the remaining prove-or-abstain cases: typealias
// slots, a local composable function type, a project type named like a library value type, a bare read from an
// unscanned star import and a member call on an unscanned receiver type.
func TestComposeRedundantComposableValuesKritCannotProve(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{
		"Values.kt": `package demo

import androidx.compose.runtime.Composable

typealias Slot = @Composable () -> Unit

@Composable
fun TakesSlot(content: Slot) {
    println("x")
}

@Composable
fun Draws() {
    val draw: @Composable () -> Unit = {}
    draw()
}

@Composable // want ComposeRedundantComposable
fun Calls() {
    val f: () -> Unit = {}
    f()
}

class Result {
    val value: Int
        @Composable get() = 1 // want ComposeRedundantComposable
}

@Composable
fun ShowResult(result: Result) = result.value

@Composable // want ComposeRedundantComposable
fun LazyRead(l: Lazy<Int>) = l.value
`,
		"Vendor.kt": `package demo.vendor

import androidx.compose.runtime.Composable
import vendor.ui.*
import vendor.widgets.VendorWidget

@Composable
fun badge() = badgeColorFromVendor

@Composable
fun Badge(widget: VendorWidget) {
    widget.renderVendorBadge()
}
`,
		"Known.kt": `package demo.known

import android.content.Context
import androidx.compose.runtime.Composable

val topLevelValue = 1

@Composable // want ComposeRedundantComposable
fun local() = topLevelValue

@Composable // want ComposeRedundantComposable
fun Packages(context: Context) {
    context.getString(1)
}
`,
	}, "ComposeRedundantComposable")
}

// TestComposeUnstableReceiverStability checks compose-lints' isStable on project receivers: star-imported Compose
// annotations, functional interfaces, value classes by their underlying type, and lookalike annotations and
// scope types from other packages.
func TestComposeUnstableReceiverStability(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{
		"Other.kt": `package com.other

annotation class Stable
annotation class StableMarker
`,
		"Wild.kt": `package demo.wild

import androidx.compose.material3.Text
import androidx.compose.runtime.*

@Stable
class WildStable(val x: Int)

@Composable
fun WildStable.Show() { Text("a") }

class WildPlain(val x: Int)

@Composable
fun WildPlain.Show() { Text("a") } // want ComposeUnstableReceiver
`,
		"Receivers.kt": `package demo

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import com.other.Stable
import com.other.StableMarker

fun interface Action {
    fun run()
}

@Composable
fun Action.Show() { Text("a") }

@FunctionalInterface
interface Legacy {
    fun run()
}

@Composable
fun Legacy.Show() { Text("a") }

interface PlainIface {
    fun run()
}

@Composable
fun PlainIface.Show() { Text("a") } // want ComposeUnstableReceiver

@JvmInline
value class Wrap(val list: MutableList<String>)

@Composable
fun Wrap.Show() { Text("a") } // want ComposeUnstableReceiver

@JvmInline
value class Id(val raw: Int)

@Composable
fun Id.Show() { Text("a") }

@Stable
class FakeStable(val x: Int)

@Composable
fun FakeStable.Show() { Text("a") } // want ComposeUnstableReceiver

@StableMarker
annotation class FakeMarker

@FakeMarker
class FakeMarked(val x: Int)

@Composable
fun FakeMarked.Show() { Text("a") } // want ComposeUnstableReceiver

@androidx.compose.runtime.StableMarker
annotation class RealMarker

@RealMarker
class RealMarked(val x: Int)

@Composable
fun RealMarked.Show() { Text("a") }

class RowScope

@Composable
fun RowScope.Item() { Text("a") } // want ComposeUnstableReceiver
`,
		"Real.kt": `package demo.real

import androidx.compose.foundation.layout.RowScope
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun RowScope.Item() { Text("a") }
`,
	}, "ComposeUnstableReceiver")
}

// TestComposeRedundantComposableInvokedProperties checks that calling a property or constructor property of a
// @Composable function type, directly, through a type alias, through `it.` or with `.invoke()`, uses the
// composition, as compose-lints' invokesComposableLambda finds; plain function types do not.
func TestComposeRedundantComposableInvokedProperties(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Tabs.kt": `package demo

import androidx.compose.runtime.Composable

typealias Slot = @Composable () -> Unit

data class TabItem(val title: String, val content: @Composable () -> Unit, val onClick: () -> Unit)

data class NavItem(val route: String, val icon: Slot)

class Screen(val content: @Composable () -> Unit, val onBack: () -> Unit) {
    @Composable
    fun Render() {
        content()
    }

    @Composable // want ComposeRedundantComposable
    fun Back() {
        onBack()
    }
}

val GlobalSlot: @Composable () -> Unit = {}

val GlobalAction: () -> Unit = {}

@Composable
fun TabContent(tab: TabItem) {
    tab.content()
}

@Composable
fun TabsContent(tabs: List<TabItem>) {
    tabs.forEach { it.content() }
}

@Composable
fun NavIcon(item: NavItem) {
    item.icon()
}

@Composable
fun NavIconInvoke(item: NavItem) {
    item.icon.invoke()
}

@Composable
fun UseGlobal() {
    GlobalSlot()
}

@Composable
fun UseGlobalInvoke() {
    GlobalSlot.invoke()
}

@Composable // want ComposeRedundantComposable
fun TabClick(tab: TabItem) {
    tab.onClick()
}

@Composable // want ComposeRedundantComposable
fun UseAction() {
    GlobalAction()
}
`}, "ComposeRedundantComposable")
}

// TestComposeReadOnlyComposableReadsSkipFunctions checks that a property read (`context.resources`) is not matched
// against project functions of the same name, which only a call can reach.
func TestComposeReadOnlyComposableReadsSkipFunctions(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{
		"Resources.kt": `package demo.res

import android.content.res.Resources
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable

class ScaffoldState(val resources: Resources)

@Composable
@ReadOnlyComposable
fun resources(): Resources = throw IllegalStateException()
`,
		"Plurals.kt": `package demo.plurals

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

@Composable // want ComposeReadOnlyComposable
fun quantityStringResource(id: Int, quantity: Int): String {
    val context = LocalContext.current
    return context.resources.getQuantityString(id, quantity)
}
`,
	}, "ComposeReadOnlyComposable")
}

// TestComposeRedundantComposableUnscannedReceiverAliases checks that a receiver from an unscanned library abstains
// without a star import, directly and through an inferred local alias.
func TestComposeRedundantComposableUnscannedReceiverAliases(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Badge.kt": `package demo.badge

import androidx.compose.runtime.Composable
import vendor.widgets.VendorWidget

@Composable
fun Direct(widget: VendorWidget) {
    widget.renderVendorBadge()
}

@Composable
fun Aliased(widget: VendorWidget) {
    val local = widget
    local.renderVendorBadge()
}

@Composable // want ComposeRedundantComposable
fun Known(items: List<String>) {
    val local = items
    local.forEach { println(it) }
}
`}, "ComposeRedundantComposable")
}

// TestComposeReadOnlyComposableLookalikeLocalFactory checks that a project function named like a Compose
// CompositionLocal factory does not make `Local*.current` a CompositionLocal read.
func TestComposeReadOnlyComposableLookalikeLocalFactory(t *testing.T) {
	checkComposeLintsWants(t, map[string]string{"Panel.kt": `package demo.panel

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

class Panel {
    val current: Int
        @Composable get() = remember { 1 }
}

fun staticCompositionLocalOf() = Panel()

val LocalPanel = staticCompositionLocalOf()

@Composable
fun panel() = LocalPanel.current
`}, "ComposeReadOnlyComposable", "ComposeRedundantComposable")
}
