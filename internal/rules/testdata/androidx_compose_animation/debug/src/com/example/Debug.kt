package com.example

import androidx.compose.animation.CustomizedLookaheadAnimationVisualDebugging
import androidx.compose.animation.ExperimentalLookaheadAnimationVisualDebugApi
import androidx.compose.animation.LookaheadAnimationVisualDebugging
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

@OptIn(ExperimentalLookaheadAnimationVisualDebugApi::class)
@Composable
fun Debugging() {
    LookaheadAnimationVisualDebugging {
        Text("x")
    }
    LookaheadAnimationVisualDebugging(isEnabled = false) { Text("y") }
    CustomizedLookaheadAnimationVisualDebugging(Color.Red) {
        Text("z")
    }
    androidx.compose.animation.LookaheadAnimationVisualDebugging(content = { Text("q") })
}
