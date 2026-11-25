package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.node.ModifierNodeElement

class TintNode : Modifier.Node()

class TintElement : ModifierNodeElement<TintNode>() {
    override fun create() = TintNode()
    override fun update(node: TintNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = other is TintElement
}
