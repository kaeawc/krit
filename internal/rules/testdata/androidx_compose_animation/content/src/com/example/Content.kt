package com.example

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
import androidx.compose.animation.core.updateTransition
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Crossfades(flag: Boolean, names: List<String>) {
    Crossfade(flag) { Text("x") }
    Crossfade(flag) { state -> Text("x") }
    Crossfade(targetState = flag, animationSpec = tween(10)) { _ -> Text("x") }
    Crossfade(flag, content = { Text("x") })
    Crossfade(flag) { if (it) Text("a") }
    Crossfade(flag) { state -> if (state) Text("a") }
    Crossfade(flag) { names.forEach { Text(it) } }
    Crossfade(flag, label = "l") { state -> names.forEach { state -> Text(state) } }
    val transition = updateTransition(flag, label = "t")
    transition.Crossfade { Text("x") }
    transition.Crossfade(contentKey = { 1 }) { state -> Text("$state") }
}

@Composable
fun AnimatedContents(count: Int) {
    AnimatedContent(count) { Text("x") }
    AnimatedContent(count) { target -> Text("x") }
    AnimatedContent(count) { target -> Text("$target") }
    AnimatedContent(targetState = count, contentKey = { 1 }) { Text("$it") }
    AnimatedContent(targetState = count, contentKey = { it % 2 }) { Text("$it") }
    AnimatedContent(count, contentKey = { state -> 0 }, content = { state -> Text("x") })
    AnimatedContent(count, label = "l") { (count) -> Text("x") }
}
