package p.locals

import androidx.compose.ui.Modifier
import androidx.compose.ui.node.CompositionLocalConsumerModifierNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.node.currentValueOf
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.platform.LocalDensity

class ReadsInAttach : Modifier.Node(), CompositionLocalConsumerModifierNode {
    var density = 1f
    override fun onAttach() {
        density = currentValueOf(LocalDensity).density
    }
    override fun onDetach() {
        density = currentValueOf(LocalDensity).density
    }
}

class ReadsInInit : Modifier.Node(), CompositionLocalConsumerModifierNode {
    init {
        currentValueOf(LocalDensity)
    }
    val lazyDensity by lazy { currentValueOf(LocalDensity) }
    val eager = currentValueOf(LocalDensity)
}

class ReadsInDraw : Modifier.Node(), CompositionLocalConsumerModifierNode, DrawModifierNode {
    override fun ContentDrawScope.draw() {
        currentValueOf(LocalDensity)
        drawContent()
    }
    override fun onAttach() {
        sideEffect { currentValueOf(LocalDensity) }
    }
}

open class Base : Modifier.Node(), CompositionLocalConsumerModifierNode

class Indirect : Base() {
    override fun onAttach() {
        currentValueOf(LocalDensity)
    }
}

class SecondaryConstructor : Modifier.Node, CompositionLocalConsumerModifierNode {
    constructor(flag: Boolean) : super() {
        currentValueOf(LocalDensity)
    }
}

class LocalFunction : Modifier.Node(), CompositionLocalConsumerModifierNode {
    override fun onAttach() {
        fun read() = currentValueOf(LocalDensity)
        read()
    }
}

class NotAConsumer : Modifier.Node() {
    override fun onAttach() {
        val d = lazy { 1 }
    }
}

fun outside(node: CompositionLocalConsumerModifierNode) {
    val value by lazy { node.currentValueOf(LocalDensity) }
}
