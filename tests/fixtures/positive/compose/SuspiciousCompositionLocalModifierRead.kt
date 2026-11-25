package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.node.CompositionLocalConsumerModifierNode
import androidx.compose.ui.node.currentValueOf
import androidx.compose.ui.platform.LocalDensity

class ScaledNode : Modifier.Node(), CompositionLocalConsumerModifierNode {
    var scale = 1f
    override fun onAttach() {
        scale = currentValueOf(LocalDensity).density
    }
}
