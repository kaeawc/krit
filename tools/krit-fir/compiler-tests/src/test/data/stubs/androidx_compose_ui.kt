// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui

import androidx.compose.runtime.Stable

@Stable
interface Modifier {
    infix fun then(other: Modifier): Modifier = TODO()

    fun any(predicate: (Element) -> Boolean): Boolean = TODO()

    fun all(predicate: (Element) -> Boolean): Boolean = TODO()

    interface Element : Modifier

    // The unnamed companion: `Modifier` as an expression is Modifier.Companion.
    companion object : Modifier
}

// Real Alignment/Horizontal/Vertical are `fun interface`s whose single method
// needs layout types (IntSize, LayoutDirection) these stubs do not model.
@Stable
interface Alignment {
    @Stable
    interface Horizontal

    @Stable
    interface Vertical

    companion object {
        val TopStart: Alignment
            get() = TODO()

        val TopCenter: Alignment
            get() = TODO()

        val TopEnd: Alignment
            get() = TODO()

        val CenterStart: Alignment
            get() = TODO()

        val Center: Alignment
            get() = TODO()

        val CenterEnd: Alignment
            get() = TODO()

        val BottomStart: Alignment
            get() = TODO()

        val BottomCenter: Alignment
            get() = TODO()

        val BottomEnd: Alignment
            get() = TODO()

        val Top: Vertical
            get() = TODO()

        val CenterVertically: Vertical
            get() = TODO()

        val Bottom: Vertical
            get() = TODO()

        val Start: Horizontal
            get() = TODO()

        val CenterHorizontally: Horizontal
            get() = TODO()

        val End: Horizontal
            get() = TODO()
    }
}
