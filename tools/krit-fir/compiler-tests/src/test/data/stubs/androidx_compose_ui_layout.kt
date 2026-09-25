// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.layout

import androidx.compose.runtime.Stable

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
