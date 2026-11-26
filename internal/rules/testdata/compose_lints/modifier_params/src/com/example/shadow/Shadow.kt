package com.example.shadow

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

// Same-package declarations shadow the default-imported kotlin.Function0 and kotlin.Comparator, so these last
// parameters are not trailing functions.
class Function0<T>

interface Comparator<T>

@Composable
fun ShadowedFunctionLast(text: String, modifier: Modifier = Modifier, block: Function0<Unit>) {
    Text(text, modifier)
}

@Composable
fun ShadowedComparatorLast(text: String, modifier: Modifier = Modifier, cmp: Comparator<String>) {
    Text(text, modifier)
}

// A GlanceModifier that is not androidx.glance's does not count as the modifier parameter.
interface GlanceModifier

@Composable
fun GlanceModifier.LookalikeRow() {
    Text("x")
}
