// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.graphics

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.Stable

// A value class over ULong. `Color(0xFF000000)` in app code is NOT this
// constructor (a Long literal does not convert to ULong): it resolves to the
// top-level `Color(color: Long)` factory function below.
@Immutable
@JvmInline
value class Color(val value: ULong) {
    val red: Float
        get() = TODO()

    val green: Float
        get() = TODO()

    val blue: Float
        get() = TODO()

    val alpha: Float
        get() = TODO()

    @Stable
    fun copy(
        alpha: Float = this.alpha,
        red: Float = this.red,
        green: Float = this.green,
        blue: Float = this.blue,
    ): Color = TODO()

    companion object {
        @Stable
        val Black: Color
            get() = TODO()

        @Stable
        val DarkGray: Color
            get() = TODO()

        @Stable
        val Gray: Color
            get() = TODO()

        @Stable
        val LightGray: Color
            get() = TODO()

        @Stable
        val White: Color
            get() = TODO()

        @Stable
        val Red: Color
            get() = TODO()

        @Stable
        val Green: Color
            get() = TODO()

        @Stable
        val Blue: Color
            get() = TODO()

        @Stable
        val Yellow: Color
            get() = TODO()

        @Stable
        val Cyan: Color
            get() = TODO()

        @Stable
        val Magenta: Color
            get() = TODO()

        @Stable
        val Transparent: Color
            get() = TODO()

        @Stable
        val Unspecified: Color
            get() = TODO()
    }
}

@Stable
fun Color(color: Long): Color = TODO()

@Stable
fun Color(color: Int): Color = TODO()

@Stable
fun Color(red: Int, green: Int, blue: Int, alpha: Int = 0xFF): Color = TODO()

@Stable
fun Color(red: Float, green: Float, blue: Float, alpha: Float = 1f): Color = TODO()

@Stable
fun Color.toArgb(): Int = TODO()

@Stable
fun Color.compositeOver(background: Color): Color = TODO()

@Stable
fun lerp(start: Color, stop: Color, fraction: Float): Color = TODO()

@Immutable
open class ColorFilter internal constructor() {
    companion object {
        @Stable
        fun tint(color: Color): ColorFilter = TODO()
    }
}

@Immutable
interface Shape

val RectangleShape: Shape
    get() = TODO()

@Immutable
abstract class Brush {
    companion object {
        @Stable
        fun verticalGradient(colors: List<Color>): Brush = TODO()

        @Stable
        fun horizontalGradient(colors: List<Color>): Brush = TODO()
    }
}
