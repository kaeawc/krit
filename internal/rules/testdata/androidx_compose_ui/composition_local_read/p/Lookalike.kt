package p.lookalike

import androidx.compose.ui.Modifier
import androidx.compose.ui.node.CompositionLocalConsumerModifierNode

fun currentValueOf(key: String): String = key

class Reader : Modifier.Node(), CompositionLocalConsumerModifierNode {
    override fun onAttach() {
        currentValueOf("x")
    }
}
