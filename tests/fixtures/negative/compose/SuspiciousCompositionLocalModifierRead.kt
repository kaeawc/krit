package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.node.CompositionLocalConsumerModifierNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.node.currentValueOf
import androidx.compose.ui.platform.LocalDensity

class ScaledNode : Modifier.Node(), CompositionLocalConsumerModifierNode, DrawModifierNode {
    override fun ContentDrawScope.draw() {
        val scale = currentValueOf(LocalDensity).density
        drawContent()
    }
}
