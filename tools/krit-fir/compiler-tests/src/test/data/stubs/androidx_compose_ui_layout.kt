// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.layout

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Constraints

interface Measurable {
    fun measure(constraints: Constraints): Placeable
}

abstract class Placeable {
    abstract val width: Int

    abstract val height: Int

    abstract class PlacementScope {
        fun Placeable.placeRelative(x: Int, y: Int): Unit = TODO()
    }
}

interface MeasureResult

interface MeasureScope {
    fun layout(width: Int, height: Int, placementBlock: Placeable.PlacementScope.() -> Unit): MeasureResult
}

fun interface MeasurePolicy {
    fun MeasureScope.measure(measurables: List<Measurable>, constraints: Constraints): MeasureResult
}

@Composable
fun Layout(content: @Composable () -> Unit, modifier: Modifier = Modifier, measurePolicy: MeasurePolicy): Unit = TODO()

@Stable
interface ContentScale {
    companion object {
        @Stable
        val Crop: ContentScale
            get() = TODO()

        @Stable
        val Fit: ContentScale
            get() = TODO()

        @Stable
        val FillBounds: ContentScale
            get() = TODO()

        @Stable
        val FillWidth: ContentScale
            get() = TODO()

        @Stable
        val FillHeight: ContentScale
            get() = TODO()

        @Stable
        val Inside: ContentScale
            get() = TODO()

        @Stable
        val None: ContentScale
            get() = TODO()
    }
}
