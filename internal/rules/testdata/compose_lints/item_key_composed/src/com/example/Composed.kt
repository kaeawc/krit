package com.example

import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.unit.dp

fun Modifier.implicit(): Modifier = composed { this }

fun Modifier.explicitThis(): Modifier = this.composed { this }

fun Modifier.chained(): Modifier = padding(1.dp).composed { this }

val fromCompanion = Modifier.composed { this }

val fromChain = Modifier.padding(2.dp).composed { this }

@Composable
fun UsesParam(modifier: Modifier) {
    val m = modifier.composed { this }
    println(m)
}

class NotModifier {
    fun composed(block: () -> Unit) = block()
}

fun NotModifier.call() = composed { }

fun lookalike(x: NotModifier) = x.composed { }
