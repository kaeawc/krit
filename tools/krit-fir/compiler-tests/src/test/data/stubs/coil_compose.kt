// Compiler-test source stubs; never packaged in the production artifact.
package coil.compose

import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.layout.ContentScale

@Composable
fun AsyncImage(
    model: Any?,
    contentDescription: String?,
    modifier: Modifier = Modifier,
    placeholder: Painter? = null,
    error: Painter? = null,
    fallback: Painter? = error,
    alignment: Alignment = Alignment.Center,
    contentScale: ContentScale = ContentScale.Fit,
    alpha: Float = 1f,
    colorFilter: ColorFilter? = null,
) {
    TODO()
}

abstract class AsyncImagePainter internal constructor() : Painter()

@Composable
fun rememberAsyncImagePainter(model: Any?, contentScale: ContentScale = ContentScale.Fit): AsyncImagePainter = TODO()
