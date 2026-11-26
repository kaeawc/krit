package com.example

import androidx.constraintlayout.compose.ConstraintSet
import androidx.constraintlayout.compose.ConstraintSetScope
import com.example.other.RefBuilder

fun build(b: RefBuilder) = with(b) {
    val (x, y) = createRefsFor("a", "b", "c")
    x + y
}

fun ConstraintSetScope.extension() {
    val (x, y) = createRefsFor("a", "b", "c")
}

val nested = ConstraintSet {
    listOf(1).forEach {
        val (x, y) = createRefsFor("a", "b", "c")
    }
}
