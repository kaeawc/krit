package com.example

import androidx.compose.ui.unit.dp
import androidx.constraintlayout.compose.ConstraintSet
import androidx.constraintlayout.compose.Dimension

fun String.withChainParams(): String = this

val spaced = ConstraintSet {
    val (a, b) = createRefsFor("a", "b")
    createHorizontalChain (a, b)
    constrain(a) {
        end.linkTo(b.start, 8.dp)
        top.linkTo(parent.top)
    }
    constrain(b) {
        start.linkTo(a.end)
    }
}

val shadowedWidth = ConstraintSet {
    val (a, b) = createRefsFor("a", "b")
    constrain(a) {
        var width = Dimension.wrapContent
        width = Dimension.matchParent
        start.linkTo(b.end)
        println(width)
    }
    constrain(b) {
        width = Dimension.matchParent
        start.linkTo(a.end)
        top.linkTo(parent.top)
    }
}

val aliased = ConstraintSet {
    val (a, middle, b) = createRefsFor("a", "middle", "b")
    val alias = middle
    createHorizontalChain(a, alias, b)
    constrain(a) {
        end.linkTo(b.start, 8.dp)
        top.linkTo(parent.top)
    }
    constrain(b) {
        start.linkTo(a.end)
    }
}

val shadowedRef = ConstraintSet {
    val (a, b) = createRefsFor("a", "b")
    run { val a = "other"; a.withChainParams() }
    createHorizontalChain(a, b)
    constrain(a) {
        end.linkTo(b.start, 8.dp)
        top.linkTo(parent.top)
    }
    constrain(b) {
        start.linkTo(a.end)
    }
}
