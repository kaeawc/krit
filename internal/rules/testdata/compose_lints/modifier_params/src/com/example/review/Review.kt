package com.example.review

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.glance.GlanceModifier

// ComposeModifierWithoutDefault: reported whatever the Modifier type's spelling, vararg included.
@Composable
fun GlanceWidget(text: String, modifier: GlanceModifier) {
}

@Composable
fun QualifiedModifier(text: String, modifier: androidx.compose.ui.Modifier) {
    Text(text, modifier)
}

@Composable
fun VarargModifier(text: String, vararg modifier: Modifier) {
    Text(text)
}

// ComposeParameterOrder: kotlin.Function* and kotlin.jvm.functions.* are trailing functions.
@Composable
fun FunctionZeroLast(text: String, modifier: Modifier = Modifier, block: Function0<Unit>) {
    Text(text, modifier)
}

@Composable
fun JvmFunctionLast(text: String, modifier: Modifier = Modifier, block: kotlin.jvm.functions.Function1<Int, Unit>) {
    Text(text, modifier)
}

@Composable
fun KotlinFunctionLast(text: String, modifier: Modifier = Modifier, block: kotlin.Function<Unit>) {
    Text(text, modifier)
}

@Composable
fun NullableFunctionNLast(text: String, modifier: Modifier = Modifier, block: Function2<Int, Int, Unit>?) {
    Text(text, modifier)
}

// kotlin.Comparator is a type alias of the @FunctionalInterface java.util.Comparator.
@Composable
fun ComparatorLast(text: String, modifier: Modifier = Modifier, cmp: Comparator<String>) {
    Text(text, modifier)
}

// The message prints each parameter's source text as written, line breaks included.
@Composable
fun MultiLineDefault(
    modifier: Modifier = Modifier,
    text: String,
    items: List<String> = listOf(
        "a",
        "b",
    ),
) {
    Text(text, modifier)
}

// ComposeModifierMissing: a GlanceModifier extension receiver is the modifier parameter.
@Composable
fun GlanceModifier.GlanceRow() {
    Text("x")
}
