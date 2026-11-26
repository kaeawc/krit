package com.example

import androidx.compose.animation.AnimatedContent as Animated
import androidx.compose.animation.Crossfade as Fade
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Aliased(flag: Boolean) {
    Fade(flag) { Text("x") }
    Fade(flag) { state -> Text("$state") }
    Animated(flag) { Text("x") }
    Animated(flag, contentKey = { 0 }) { Text("$it") }
}
