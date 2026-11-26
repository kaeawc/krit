package com.example

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColor
import androidx.compose.animation.core.Transition
import androidx.compose.animation.core.animateDp
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.rememberTransition
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.updateTransition
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

@Composable
fun Transitions(flag: Boolean, other: Boolean) {
    val transition = updateTransition(flag, label = "t")
    transition.animateFloat(label = "a") { if (other) 1f else 0f }
    transition.animateFloat(label = "a") { state -> if (other) 1f else 0f }
    transition.animateFloat(label = "a") { if (it) 1f else 0f }
    transition.animateFloat(transitionSpec = { spring() }, label = "a") { _ -> 1f }
    transition.animateFloat(transitionSpec = { spring() }, label = "a", targetValueByState = { 1f })
    transition.animateDp(label = "d") { 1.dp }
    transition.animateColor(label = "c") { Color.Red }
    val remembered = rememberTransition(MutableTransitionState(flag), label = "r")
    remembered.animateFloat(label = "a") { 1f }
    updateTransition(flag, label = "x").animateFloat(label = "a") { 1f }
}

@Composable
fun Transition<Boolean>.Extension() {
    animateFloat(label = "a") { 1f }
    this.animateFloat(label = "a") { 1f }
    animateFloat(label = "a") { s -> if (s) 1f else 0f }
}

@Composable
fun Param(t: Transition<Int>) {
    t.animateFloat(label = "a") { 2f }
    t.animateFloat(label = "a") { state -> state.toFloat() }
}

@Composable
fun Visibility(flag: Boolean) {
    AnimatedVisibility(flag) {
        transition.animateFloat(label = "a") { 1f }
    }
}

@Composable
fun <S> Transition<S>.animateCustom(block: (S) -> Float): Float = 0f

@Composable
fun Custom(t: Transition<Int>) {
    t.animateCustom { 1f }
}
