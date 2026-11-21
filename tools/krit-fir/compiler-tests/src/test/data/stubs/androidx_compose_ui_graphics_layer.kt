package androidx.compose.ui.graphics

import androidx.compose.ui.Modifier

interface GraphicsLayerScope {
    var alpha: Float
}

fun Modifier.graphicsLayer(block: GraphicsLayerScope.() -> Unit): Modifier = TODO()
