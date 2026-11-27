package com.example

import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.BoxWithConstraintsScope
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun BoxWithConstraintsScope.Helper() { Text("$maxWidth") }

val BoxWithConstraintsScope.half get() = maxWidth / 2

@Composable
fun Boxes(content: @Composable BoxWithConstraintsScope.() -> Unit) {
    BoxWithConstraints { Text("x") }
    BoxWithConstraints { Text("$maxWidth") }
    BoxWithConstraints { if (maxHeight.value > 1f) Text("y") }
    BoxWithConstraints { Text("${constraints.maxWidth}") }
    BoxWithConstraints { Helper() }
    BoxWithConstraints { Text("$half") }
    BoxWithConstraints { println(this) }
    BoxWithConstraints(Modifier) { Text("x") }
    BoxWithConstraints(content = { Text("x") })
    BoxWithConstraints(content = content)
    BoxWithConstraints { content() }
    BoxWithConstraints { val w = minWidth; Text("$w") }
}
