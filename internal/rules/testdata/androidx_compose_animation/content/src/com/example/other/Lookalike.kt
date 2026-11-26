package com.example.other

import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Crossfade(flag: Boolean, content: @Composable (Boolean) -> Unit) = content(flag)

@Composable
fun AnimatedContent(count: Int, content: @Composable (Int) -> Unit) = content(count)

@Composable
fun Uses() {
    Crossfade(true) { Text("x") }
    AnimatedContent(1) { Text("x") }
}
