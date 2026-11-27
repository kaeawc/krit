package com.example

import android.graphics.Rect
import androidx.compose.animation.core.Animatable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.offset as off
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@Composable
fun Review(input: Dp, r: Rect) {
    var s by rememberSaveable { mutableStateOf(0.dp) }
    val updated by rememberUpdatedState(input)
    val a = remember { Animatable(0f) }
    Box(Modifier.off(s, 0.dp))
    Box(Modifier.off(updated, 0.dp))
    r.offset(a.value.toInt(), 0)
}
