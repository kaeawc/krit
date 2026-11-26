package com.example.other

import androidx.compose.runtime.Composable

@Composable
fun LookaheadAnimationVisualDebugging(content: @Composable () -> Unit) = content()

@Composable
fun Uses() {
    LookaheadAnimationVisualDebugging {}
}
