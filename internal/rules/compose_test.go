package rules_test

import "testing"

func TestComposeColumnRowInScrollable_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeColumnRowInScrollable", `
package test
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(items: List<String>) {
    Column(Modifier.verticalScroll(rememberScrollState())) {
        Header()
        LazyColumn {
            items(items) { item -> RowItem(item) }
        }
    }

    Row(Modifier.horizontalScroll(rememberScrollState())) {
        Sidebar()
        LazyRow {
            items(items) { item -> Chip(item) }
        }
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeColumnRowInScrollable_Negative(t *testing.T) {
	findings := runRuleByName(t, "ComposeColumnRowInScrollable", `
package test
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(items: List<String>) {
    LazyColumn {
        item { Header() }
        items(items) { item -> RowItem(item) }
    }

    Row(Modifier.horizontalScroll(rememberScrollState())) {
        items.forEach { item -> Chip(item) }
    }

    LazyRow {
        items(items) { item -> Chip(item) }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeDerivedStateMisuse_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeDerivedStateMisuse", `
package test
import androidx.compose.runtime.*

@Composable
fun Example() {
    val count by remember { mutableStateOf(0) }
    val isPositive by remember { derivedStateOf { count > 0 } }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeDerivedStateMisuse_Negative(t *testing.T) {
	findings := runRuleByName(t, "ComposeDerivedStateMisuse", `
package test
import androidx.compose.runtime.*

@Composable
fun Example() {
    val count by remember { mutableStateOf(0) }
    val bucket by remember { derivedStateOf { count / 10 } }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeLambdaCapturesUnstableState_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeLambdaCapturesUnstableState", `
package test
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Example(vm: Vm, users: List<User>) {
    LazyColumn {
        items(users) { user ->
            Button(onClick = { vm.select(user) }) { Text(user.name) }
        }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeLambdaCapturesUnstableState_Negative(t *testing.T) {
	findings := runRuleByName(t, "ComposeLambdaCapturesUnstableState", `
package test
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Example(vm: Vm, users: List<User>) {
    LazyColumn {
        items(users) { user ->
            val onClick = remember(user) { { vm.select(user) } }
            Button(onClick = onClick) { Text(user.name) }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeLambdaCapturesUnstableState_NegativePropertyRead(t *testing.T) {
	findings := runRuleByName(t, "ComposeLambdaCapturesUnstableState", `
package test
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Example(vm: Vm, users: List<User>) {
    LazyColumn {
        items(users) { user ->
            Button(onClick = { vm.select(user.id) }) { Text(user.name) }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Positive_FillMaxWidthAfterSize(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.size(48.dp).fillMaxWidth())
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Positive_FillMaxHeightAfterSize(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.size(48.dp).fillMaxHeight())
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Positive_FillMaxSizeAfterSize(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.size(48.dp).fillMaxSize())
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Negative_FillBeforeHeight(t *testing.T) {
	// fillMaxWidth() followed by height() is fine — no size() call in the chain.
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.fillMaxWidth().height(48.dp))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Negative_SizeAfterFill(t *testing.T) {
	// size() AFTER fillMax* is not the target pattern (the fillMax is earlier;
	// the explicit size is what actually applies).
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.fillMaxWidth().size(48.dp))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Negative_SizeAloneNoFill(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.size(48.dp))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierFillAfterSize_Negative_UnrelatedChain(t *testing.T) {
	// padding and clickable are unrelated to the size/fillMax interaction.
	findings := runRuleByName(t, "ComposeModifierFillAfterSize", `
package test
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.padding(16.dp).clickable { })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierBackgroundAfterClip_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierBackgroundAfterClip", `
package test
import androidx.compose.foundation.background
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.background(Color.Red).clip(RoundedCornerShape(8.dp)))
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierBackgroundAfterClip_Negative_CorrectOrder(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierBackgroundAfterClip", `
package test
import androidx.compose.foundation.background
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.clip(RoundedCornerShape(8.dp)).background(Color.Red))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierBackgroundAfterClip_Negative_BackgroundOnly(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierBackgroundAfterClip", `
package test
import androidx.compose.foundation.background
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color

@Composable
fun Example() {
    Box(Modifier.background(Color.Red))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierClickableBeforePadding_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierClickableBeforePadding", `
package test
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.clickable { }.padding(16.dp))
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierClickableBeforePadding_Negative_CorrectOrder(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierClickableBeforePadding", `
package test
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.padding(16.dp).clickable { })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierClickableBeforePadding_Negative_PaddingAlone(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierClickableBeforePadding", `
package test
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example() {
    Box(Modifier.padding(16.dp))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewAnnotationMissing_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewAnnotationMissing", `
package test
import androidx.compose.runtime.Composable

@Composable
fun FooPreview() { Foo() }
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewAnnotationMissing_Negative_HasPreviewAnnotation(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewAnnotationMissing", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

@Preview
@Composable
fun FooPreview() { Foo() }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewAnnotationMissing_Negative_NotComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewAnnotationMissing", `
package test

fun FooPreview() { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewAnnotationMissing_Negative_NameDoesNotEndInPreview(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewAnnotationMissing", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Foo() { Bar() }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableDefaultArgument_Positive_MutableListOf(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableDefaultArgument", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Foo(items: MutableList<String> = mutableListOf()) { }
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableDefaultArgument_Positive_MutableMapOf(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableDefaultArgument", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Foo(entries: MutableMap<String, Int> = mutableMapOf()) { }
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableDefaultArgument_Negative_EmptyListDefault(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableDefaultArgument", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Foo(items: List<String> = emptyList()) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableDefaultArgument_Negative_NotComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableDefaultArgument", `
package test

fun Foo(items: MutableList<String> = mutableListOf()) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableDefaultArgument_Negative_NoDefault(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableDefaultArgument", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Foo(items: MutableList<String>) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeStringResourceInsideLambda_Positive_OnClick(t *testing.T) {
	findings := runRuleByName(t, "ComposeStringResourceInsideLambda", `
package test
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Example() {
    Button(onClick = {
        Log.d("TAG", stringResource(R.string.click_label))
    }) { Text("Click") }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeStringResourceInsideLambda_Negative_HoistedAbove(t *testing.T) {
	findings := runRuleByName(t, "ComposeStringResourceInsideLambda", `
package test
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Example() {
    val label = stringResource(R.string.click_label)
    Button(onClick = { Log.d("TAG", label) }) { Text("Click") }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeStringResourceInsideLambda_Negative_ContentLambda(t *testing.T) {
	// The trailing-lambda content slot of Button is a @Composable lambda;
	// stringResource() is legal there.
	findings := runRuleByName(t, "ComposeStringResourceInsideLambda", `
package test
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Example() {
    Button(onClick = { }) { Text(stringResource(R.string.click_label)) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberWithoutKey_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberWithoutKey", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Chart(dataset: List<Int>) {
    val series = remember { buildSeries(dataset) }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberWithoutKey_Negative_HasKey(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberWithoutKey", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Chart(dataset: List<Int>) {
    val series = remember(dataset) { buildSeries(dataset) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberWithoutKey_Negative_NoParamReference(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberWithoutKey", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Counter() {
    val answer = remember { 42 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberWithoutKey_Negative_NoEnclosingParam(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberWithoutKey", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Example() {
    val label = remember { "hello" }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeLaunchedEffectWithoutKeys_Positive_Unit(t *testing.T) {
	findings := runRuleByName(t, "ComposeLaunchedEffectWithoutKeys", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun Example(userId: String) {
    LaunchedEffect(Unit) {
        fetch(userId)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeLaunchedEffectWithoutKeys_Positive_True(t *testing.T) {
	findings := runRuleByName(t, "ComposeLaunchedEffectWithoutKeys", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun Example(userId: String) {
    LaunchedEffect(true) {
        fetch(userId)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeLaunchedEffectWithoutKeys_Negative_KeyedCorrectly(t *testing.T) {
	findings := runRuleByName(t, "ComposeLaunchedEffectWithoutKeys", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun Example(userId: String) {
    LaunchedEffect(userId) {
        fetch(userId)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeLaunchedEffectWithoutKeys_Negative_NoParamReference(t *testing.T) {
	findings := runRuleByName(t, "ComposeLaunchedEffectWithoutKeys", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun Heartbeat() {
    LaunchedEffect(Unit) {
        tick()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableStateInComposition_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableStateInComposition", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf

@Composable
fun Counter() {
    val count = mutableStateOf(0)
    Text(count.value.toString())
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableStateInComposition_Negative_RememberDelegate(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableStateInComposition", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue

@Composable
fun Counter() {
    var count by remember { mutableStateOf(0) }
    Text(count.toString())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableStateInComposition_Negative_RememberAssignment(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableStateInComposition", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember

@Composable
fun Counter() {
    val state = remember { mutableStateOf(0) }
    Text(state.value.toString())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeMutableStateInComposition_Negative_NotComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposeMutableStateInComposition", `
package test
import androidx.compose.runtime.mutableStateOf

fun Counter() {
    val count = mutableStateOf(0)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeStatefulDefaultParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeStatefulDefaultParameter", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Counter(state: CounterState = CounterState()) { }
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeStatefulDefaultParameter_Negative_RememberHelper(t *testing.T) {
	findings := runRuleByName(t, "ComposeStatefulDefaultParameter", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Counter(state: CounterState = rememberCounterState()) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeStatefulDefaultParameter_Negative_ModifierDefault(t *testing.T) {
	// Modifier as a bare identifier (the companion object) is NOT a call.
	findings := runRuleByName(t, "ComposeStatefulDefaultParameter", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(mod: Modifier = Modifier) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeStatefulDefaultParameter_Negative_LowercaseFactory(t *testing.T) {
	// Factory functions start with lowercase, so they are NOT flagged.
	findings := runRuleByName(t, "ComposeStatefulDefaultParameter", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Example(items: List<String> = emptyList()) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeStatefulDefaultParameter_Negative_NotComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposeStatefulDefaultParameter", `
package test

fun Counter(state: CounterState = CounterState()) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewWithBackingState_Positive_HiltViewModel(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewWithBackingState", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel

@Preview
@Composable
fun FooPreview() {
    val vm: FooViewModel = hiltViewModel()
    Foo(vm)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewWithBackingState_Positive_CollectAsState(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewWithBackingState", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.tooling.preview.Preview

@Preview
@Composable
fun FooPreview() {
    val state by vm.state.collectAsState()
    Foo(state)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewWithBackingState_Negative_FakeData(t *testing.T) {
	findings := runRuleByName(t, "ComposePreviewWithBackingState", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

@Preview
@Composable
fun FooPreview() {
    Foo(FakeFooState())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposePreviewWithBackingState_Negative_NotPreview(t *testing.T) {
	// Without @Preview, hiltViewModel() is legitimate.
	findings := runRuleByName(t, "ComposePreviewWithBackingState", `
package test
import androidx.compose.runtime.Composable
import androidx.hilt.navigation.compose.hiltViewModel

@Composable
fun RealScreen() {
    val vm: FooViewModel = hiltViewModel()
    Foo(vm)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeDisposableEffectMissingDispose_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeDisposableEffectMissingDispose", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect

@Composable
fun Example(listener: Listener) {
    DisposableEffect(listener) {
        source.addListener(listener)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeDisposableEffectMissingDispose_Negative_HasOnDispose(t *testing.T) {
	findings := runRuleByName(t, "ComposeDisposableEffectMissingDispose", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect

@Composable
fun Example(listener: Listener) {
    DisposableEffect(listener) {
        source.addListener(listener)
        onDispose { source.removeListener(listener) }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeDisposableEffectMissingDispose_Negative_OnlyOnDispose(t *testing.T) {
	findings := runRuleByName(t, "ComposeDisposableEffectMissingDispose", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect

@Composable
fun Example(listener: Listener) {
    DisposableEffect(listener) {
        onDispose { cleanup() }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierPassedThenChained_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierPassedThenChained", `
package test
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card(modifier: Modifier = Modifier) {
    Box(Modifier.fillMaxSize()) { }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierPassedThenChained_Negative_UsesModifier(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierPassedThenChained", `
package test
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize()) { }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierPassedThenChained_Negative_NoModifierParam(t *testing.T) {
	findings := runRuleByName(t, "ComposeModifierPassedThenChained", `
package test
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card() {
    Box(Modifier.fillMaxSize()) { }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeModifierPassedThenChained_Negative_ModifierPassedPlain(t *testing.T) {
	// If the author passes `modifier` through unchained, the parameter is used.
	findings := runRuleByName(t, "ComposeModifierPassedThenChained", `
package test
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card(modifier: Modifier = Modifier) {
    Box(modifier) { }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeSideEffectInComposition_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeSideEffectInComposition", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Screen(vm: VM) {
    vm.tracker.seen = true
    Content(vm.state)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeSideEffectInComposition_Negative_InLaunchedEffect(t *testing.T) {
	findings := runRuleByName(t, "ComposeSideEffectInComposition", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun Screen(vm: VM) {
    LaunchedEffect(Unit) { vm.tracker.seen = true }
    Content(vm.state)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeSideEffectInComposition_Negative_InSideEffect(t *testing.T) {
	findings := runRuleByName(t, "ComposeSideEffectInComposition", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect

@Composable
fun Screen(vm: VM) {
    SideEffect { vm.tracker.seen = true }
    Content(vm.state)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeSideEffectInComposition_Negative_NamedEventCallback(t *testing.T) {
	findings := runRuleByName(t, "ComposeSideEffectInComposition", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Screen(vm: VM) {
    Content(
        onConfirm = {
            vm.tracker.seen = true
        }
    )
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for event callback assignment, got %d: %v", len(findings), findings)
	}
}

func TestComposeSideEffectInComposition_Positive_NamedContentLambda(t *testing.T) {
	findings := runRuleByName(t, "ComposeSideEffectInComposition", `
package test
import androidx.compose.runtime.Composable

@Composable
fun Screen(vm: VM) {
    Column(content = {
        vm.tracker.seen = true
    })
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for composition content lambda assignment, got %d: %v", len(findings), findings)
	}
}

func TestComposeSideEffectInComposition_Negative_NotComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposeSideEffectInComposition", `
package test

fun Screen(vm: VM) {
    vm.tracker.seen = true
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeUnstableParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeUnstableParameter", `
package test
import androidx.compose.runtime.Composable

@Composable
fun UserList(users: List<User>) { }
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeUnstableParameter_Negative_ImmutableList(t *testing.T) {
	findings := runRuleByName(t, "ComposeUnstableParameter", `
package test
import androidx.compose.runtime.Composable
import kotlinx.collections.immutable.ImmutableList

@Composable
fun UserList(users: ImmutableList<User>) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeUnstableParameter_Negative_WrappedInStableClass(t *testing.T) {
	findings := runRuleByName(t, "ComposeUnstableParameter", `
package test
import androidx.compose.runtime.Composable

@Composable
fun UserList(users: Users) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeUnstableParameter_Negative_NotComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposeUnstableParameter", `
package test

fun UserList(users: List<User>) { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberSaveableNonParcelable_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberSaveableNonParcelable", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.saveable.rememberSaveable

@Composable
fun Example() {
    val state = rememberSaveable { MyState() }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberSaveableNonParcelable_Negative_PrimitiveInLambda(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberSaveableNonParcelable", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.saveable.rememberSaveable

@Composable
fun Example() {
    val count = rememberSaveable { 0 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRememberSaveableNonParcelable_Negative_WithSaver(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberSaveableNonParcelable", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.saveable.rememberSaveable

@Composable
fun Example() {
    val state = rememberSaveable(saver = MyStateSaver) { MyState() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}
