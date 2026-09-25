// Compiler-test source stubs; never packaged in the production artifact.
package android.graphics.drawable

import android.graphics.Canvas
import android.graphics.ColorFilter

abstract class Drawable {
    open val intrinsicWidth: Int
        get() = TODO()

    open val intrinsicHeight: Int
        get() = TODO()

    abstract fun draw(canvas: Canvas)

    abstract fun setAlpha(alpha: Int)

    abstract fun setColorFilter(colorFilter: ColorFilter?)

    // Overridden by app code, so it stays a function rather than a property.
    @Deprecated("Deprecated in Java")
    abstract fun getOpacity(): Int

    fun setBounds(left: Int, top: Int, right: Int, bottom: Int) {
        TODO()
    }

    open fun setTint(tintColor: Int) {
        TODO()
    }
}

open class ColorDrawable(color: Int) : Drawable() {
    override fun draw(canvas: Canvas) {
        TODO()
    }

    override fun setAlpha(alpha: Int) {
        TODO()
    }

    override fun setColorFilter(colorFilter: ColorFilter?) {
        TODO()
    }

    @Deprecated("Deprecated in Java")
    override fun getOpacity(): Int = TODO()
}
