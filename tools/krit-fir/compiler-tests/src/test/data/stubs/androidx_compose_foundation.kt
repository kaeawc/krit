// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.Dp

@Composable
fun Image(
    painter: Painter,
    contentDescription: String?,
    modifier: Modifier = Modifier,
    alignment: Alignment = Alignment.Center,
    contentScale: ContentScale = ContentScale.Fit,
    alpha: Float = 1f,
    colorFilter: ColorFilter? = null,
) {
    TODO()
}

@Composable
fun Image(
    imageVector: ImageVector,
    contentDescription: String?,
    modifier: Modifier = Modifier,
    alignment: Alignment = Alignment.Center,
    contentScale: ContentScale = ContentScale.Fit,
    alpha: Float = 1f,
    colorFilter: ColorFilter? = null,
) {
    TODO()
}

fun Modifier.background(color: Color, shape: Shape = RectangleShape): Modifier = TODO()

fun Modifier.background(brush: Brush, shape: Shape = RectangleShape, alpha: Float = 1.0f): Modifier = TODO()

fun Modifier.border(width: Dp, color: Color, shape: Shape = RectangleShape): Modifier = TODO()

// The a11y-relevant overload: label and role are what accessibility rules read.
fun Modifier.clickable(
    enabled: Boolean = true,
    onClickLabel: String? = null,
    role: Role? = null,
    onClick: () -> Unit,
): Modifier = TODO()

fun Modifier.combinedClickable(
    enabled: Boolean = true,
    onClickLabel: String? = null,
    role: Role? = null,
    onLongClickLabel: String? = null,
    onLongClick: (() -> Unit)? = null,
    onDoubleClick: (() -> Unit)? = null,
    onClick: () -> Unit,
): Modifier = TODO()

@Stable
class ScrollState(initial: Int) {
    val value: Int
        get() = TODO()

    val maxValue: Int
        get() = TODO()

    suspend fun animateScrollTo(value: Int) {
        TODO()
    }

    suspend fun scrollTo(value: Int): Float = TODO()
}

@Composable
fun rememberScrollState(initial: Int = 0): ScrollState = TODO()

fun Modifier.verticalScroll(
    state: ScrollState,
    enabled: Boolean = true,
    reverseScrolling: Boolean = false,
): Modifier = TODO()

fun Modifier.horizontalScroll(
    state: ScrollState,
    enabled: Boolean = true,
    reverseScrolling: Boolean = false,
): Modifier = TODO()
