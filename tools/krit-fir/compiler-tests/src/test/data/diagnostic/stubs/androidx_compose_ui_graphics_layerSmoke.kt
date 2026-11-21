package stubs

import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer

fun GraphicsLayerSmoke(modifier: Modifier): Modifier = modifier.graphicsLayer { alpha = 0.5f }
