// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.unit

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.Stable

@Immutable
@JvmInline
value class Dp(val value: Float) : Comparable<Dp> {
    @Stable
    operator fun plus(other: Dp): Dp = TODO()

    @Stable
    operator fun minus(other: Dp): Dp = TODO()

    @Stable
    operator fun unaryMinus(): Dp = TODO()

    @Stable
    operator fun div(other: Float): Dp = TODO()

    @Stable
    operator fun div(other: Int): Dp = TODO()

    @Stable
    operator fun div(other: Dp): Float = TODO()

    @Stable
    operator fun times(other: Float): Dp = TODO()

    @Stable
    operator fun times(other: Int): Dp = TODO()

    @Stable
    override operator fun compareTo(other: Dp): Int = TODO()

    companion object {
        @Stable
        val Hairline: Dp
            get() = TODO()

        @Stable
        val Infinity: Dp
            get() = TODO()

        @Stable
        val Unspecified: Dp
            get() = TODO()
    }
}

@Stable
inline val Int.dp: Dp
    get() = TODO()

@Stable
inline val Double.dp: Dp
    get() = TODO()

@Stable
inline val Float.dp: Dp
    get() = TODO()

@Stable
operator fun Int.times(other: Dp): Dp = TODO()

@Stable
operator fun Float.times(other: Dp): Dp = TODO()

@Immutable
@JvmInline
value class TextUnit(val packedValue: Long) {
    val value: Float
        get() = TODO()

    companion object {
        @Stable
        val Unspecified: TextUnit
            get() = TODO()
    }
}

@Stable
inline val Int.sp: TextUnit
    get() = TODO()

@Stable
inline val Double.sp: TextUnit
    get() = TODO()

@Stable
inline val Float.sp: TextUnit
    get() = TODO()

@Immutable
@JvmInline
value class DpSize(val packedValue: Long) {
    val width: Dp
        get() = TODO()

    val height: Dp
        get() = TODO()
}

@Stable
fun DpSize(width: Dp, height: Dp): DpSize = TODO()
