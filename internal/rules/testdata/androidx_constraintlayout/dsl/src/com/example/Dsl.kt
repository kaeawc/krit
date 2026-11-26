package com.example

import androidx.compose.ui.unit.dp
import androidx.constraintlayout.compose.ConstraintSet
import androidx.constraintlayout.compose.Dimension
import androidx.constraintlayout.compose.MotionScene

val refs = ConstraintSet {
    val (a, b) = createRefsFor("a", "b", "c")
    val (c, d, e) = createRefsFor("c", "d")
    val (f, g) = createRefsFor("f", "g")
    val ids = arrayOf("x", "y")
    val (h, i) = createRefsFor(*ids)
    val (j) = createRefsFor("j", 1)
}

val scene = MotionScene {
    val (a, b) = createRefsFor("a")
}

val matchParent = ConstraintSet {
    val (text, button) = createRefsFor("text", "button")
    constrain(text) {
        width = Dimension.matchParent
        start.linkTo(parent.start)
        end.linkTo(parent.end)
    }
    constrain(button) {
        width = Dimension.matchParent
        start.linkTo(text.end)
    }
    constrain(text) {
        height = Dimension.matchParent
        top.linkTo(button.bottom)
    }
    constrain(button) {
        height = Dimension.matchParent
        centerVerticallyTo(text)
    }
    constrain(text) {
        width = Dimension.matchParent
        centerTo(parent)
        centerHorizontallyTo(parent)
    }
    constrain(button) {
        width = Dimension.matchParent
        centerHorizontallyTo(text)
        top.linkTo(text.bottom)
    }
}

val chains = ConstraintSet {
    val (a, b, c) = createRefsFor("a", "b", "c")
    createHorizontalChain(a, b, c)
    constrain(a) {
        end.linkTo(b.start, 8.dp)
        start.linkTo(parent.start, 4.dp, 2.dp)
    }
    constrain(b) {
        end.linkTo(c.start, margin = 8.dp)
        top.linkTo(parent.top, 8.dp)
    }
    constrain(c) {
        start.linkTo(b.end)
    }
    val (d, e) = createRefsFor("d", "e")
    createVerticalChain(d.withChainParams(topMargin = 4.dp), e)
    constrain(d) {
        bottom.linkTo(e.top, 8.dp)
    }
    constrain(e) {
        top.linkTo(d.bottom, 8.dp)
    }
}
