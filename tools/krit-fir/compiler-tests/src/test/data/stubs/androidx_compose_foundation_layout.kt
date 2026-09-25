// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation.layout

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@DslMarker
annotation class LayoutScopeMarker

@LayoutScopeMarker
@Immutable
interface ColumnScope {
    @Stable
    fun Modifier.weight(weight: Float, fill: Boolean = true): Modifier

    @Stable
    fun Modifier.align(alignment: Alignment.Horizontal): Modifier
}

@LayoutScopeMarker
@Immutable
interface RowScope {
    @Stable
    fun Modifier.weight(weight: Float, fill: Boolean = true): Modifier

    @Stable
    fun Modifier.align(alignment: Alignment.Vertical): Modifier
}

@LayoutScopeMarker
@Immutable
interface BoxScope {
    @Stable
    fun Modifier.align(alignment: Alignment): Modifier

    @Stable
    fun Modifier.matchParentSize(): Modifier
}

@Immutable
object Arrangement {
    @Stable
    interface Horizontal

    @Stable
    interface Vertical

    @Stable
    interface HorizontalOrVertical : Horizontal, Vertical

    @Stable
    val Start: Horizontal
        get() = TODO()

    @Stable
    val End: Horizontal
        get() = TODO()

    @Stable
    val Top: Vertical
        get() = TODO()

    @Stable
    val Bottom: Vertical
        get() = TODO()

    @Stable
    val Center: HorizontalOrVertical
        get() = TODO()

    @Stable
    val SpaceBetween: HorizontalOrVertical
        get() = TODO()

    @Stable
    val SpaceAround: HorizontalOrVertical
        get() = TODO()

    @Stable
    val SpaceEvenly: HorizontalOrVertical
        get() = TODO()

    @Stable
    fun spacedBy(space: Dp): HorizontalOrVertical = TODO()
}

// Column/Row/Box are inline and take scoped @Composable content lambdas.
@Composable
inline fun Column(
    modifier: Modifier = Modifier,
    verticalArrangement: Arrangement.Vertical = Arrangement.Top,
    horizontalAlignment: Alignment.Horizontal = Alignment.Start,
    content: @Composable ColumnScope.() -> Unit,
) {
    TODO()
}

@Composable
inline fun Row(
    modifier: Modifier = Modifier,
    horizontalArrangement: Arrangement.Horizontal = Arrangement.Start,
    verticalAlignment: Alignment.Vertical = Alignment.Top,
    content: @Composable RowScope.() -> Unit,
) {
    TODO()
}

@Composable
inline fun Box(
    modifier: Modifier = Modifier,
    contentAlignment: Alignment = Alignment.TopStart,
    propagateMinConstraints: Boolean = false,
    content: @Composable BoxScope.() -> Unit,
) {
    TODO()
}

@Composable
fun Box(modifier: Modifier) {
    TODO()
}

@Composable
fun Spacer(modifier: Modifier) {
    TODO()
}

@Stable
interface PaddingValues

@Stable
fun PaddingValues(all: Dp): PaddingValues = TODO()

@Stable
fun PaddingValues(horizontal: Dp = 0.dp, vertical: Dp = 0.dp): PaddingValues = TODO()

@Stable
fun PaddingValues(start: Dp = 0.dp, top: Dp = 0.dp, end: Dp = 0.dp, bottom: Dp = 0.dp): PaddingValues = TODO()

@Stable
fun Modifier.padding(all: Dp): Modifier = TODO()

@Stable
fun Modifier.padding(horizontal: Dp = 0.dp, vertical: Dp = 0.dp): Modifier = TODO()

@Stable
fun Modifier.padding(start: Dp = 0.dp, top: Dp = 0.dp, end: Dp = 0.dp, bottom: Dp = 0.dp): Modifier = TODO()

@Stable
fun Modifier.padding(paddingValues: PaddingValues): Modifier = TODO()

@Stable
fun Modifier.fillMaxWidth(fraction: Float = 1f): Modifier = TODO()

@Stable
fun Modifier.fillMaxHeight(fraction: Float = 1f): Modifier = TODO()

@Stable
fun Modifier.fillMaxSize(fraction: Float = 1f): Modifier = TODO()

@Stable
fun Modifier.width(width: Dp): Modifier = TODO()

@Stable
fun Modifier.height(height: Dp): Modifier = TODO()

@Stable
fun Modifier.size(size: Dp): Modifier = TODO()

@Stable
fun Modifier.size(width: Dp, height: Dp): Modifier = TODO()

@Stable
fun Modifier.wrapContentSize(align: Alignment = Alignment.Center, unbounded: Boolean = false): Modifier = TODO()

@Stable
fun Modifier.offset(x: Dp = 0.dp, y: Dp = 0.dp): Modifier = TODO()
