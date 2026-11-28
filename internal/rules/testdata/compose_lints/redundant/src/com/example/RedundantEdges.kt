package com.example.edges

import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.runtime.Composable

private val shapes: Shapes
    @Composable
    get() = MaterialTheme.shapes.copy(large = CircleShape)

@Composable
fun Graph(count: Int, label: @Composable (Int) -> Unit, bar: @Composable Scope.(Int) -> Unit) {
    val labels = @Composable { repeat(count) { label(it) } }
    val bars = @Composable { repeat(count) { Scope.bar(it) } }
    println(labels)
    println(bars)
}

object Scope

@Composable
fun UsesLambdaParam(items: List<Int>): Int {
    var total = 0
    items.forEach { item -> total += item }
    return total
}
