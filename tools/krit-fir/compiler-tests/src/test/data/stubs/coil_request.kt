// Compiler-test source stubs; never packaged in the production artifact.
package coil.request

import android.content.Context

class ImageRequest private constructor() {
    val context: Context
        get() = TODO()

    val data: Any
        get() = TODO()

    fun newBuilder(context: Context = this.context): Builder = TODO()

    class Builder(context: Context) {
        fun data(data: Any?): Builder = TODO()

        fun crossfade(enable: Boolean): Builder = TODO()

        fun crossfade(durationMillis: Int): Builder = TODO()

        fun placeholder(drawableResId: Int): Builder = TODO()

        fun error(drawableResId: Int): Builder = TODO()

        fun memoryCacheKey(key: String?): Builder = TODO()

        fun build(): ImageRequest = TODO()
    }
}
