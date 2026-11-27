package com.example

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.absoluteOffset
import androidx.compose.foundation.layout.offset
import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp

@Composable
fun Offsets(state: State<Dp>, plain: Dp, mutable: MutableState<Dp>) {
    val x = remember { mutableStateOf(0.dp) }
    val delegated by remember { mutableStateOf(0.dp) }
    val animated by animateDpAsState(10.dp)
    val anim = remember { Animatable(0f) }
    Box(Modifier.offset(x.value, 0.dp))
    Box(Modifier.offset(state.value, plain))
    Box(Modifier.offset(delegated, 0.dp))
    Box(Modifier.offset(animated, animated))
    Box(Modifier.absoluteOffset(mutable.value, 0.dp))
    Box(Modifier.offset(anim.value.dp, 0.dp))
    Box(Modifier.offset(plain, 1.dp))
    Box(Modifier.offset(x = plain))
    Box(Modifier.offset(y = delegated))
    Box(Modifier.offset { IntOffset(0, 0) })
    Box(Modifier.offset { IntOffset(x.value.roundToPx(), 0) })
    val snapshot = x.value
    Box(Modifier.offset(snapshot, 0.dp))
}
