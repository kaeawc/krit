package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.node.ModifierNodeElement
import androidx.compose.ui.platform.InspectorInfo

class TintNode : Modifier.Node()

data class TintElement(val alpha: Float) : ModifierNodeElement<TintNode>() {
    override fun create() = TintNode()
    override fun update(node: TintNode) {}
    override fun InspectorInfo.inspectableProperties() {
        name = "tint"
    }
}

abstract class NotCompose<T>

class Lookalike : NotCompose<Int>()
