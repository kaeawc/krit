// Compiler-test source stubs; never packaged in the production artifact.
package android.graphics

open class Paint {
    constructor()

    constructor(flags: Int)

    open var color: Int
        get() = TODO()
        set(value) = TODO()

    open var isAntiAlias: Boolean
        get() = TODO()
        set(value) = TODO()

    open var textSize: Float
        get() = TODO()
        set(value) = TODO()

    open var strokeWidth: Float
        get() = TODO()
        set(value) = TODO()

    open var style: Style
        get() = TODO()
        set(value) = TODO()

    enum class Style {
        FILL,
        STROKE,
        FILL_AND_STROKE,
    }

    companion object {
        const val ANTI_ALIAS_FLAG: Int = 1
    }
}

// Java class of static color constants and helpers.
object Color {
    const val BLACK: Int = -16777216
    const val WHITE: Int = -1
    const val RED: Int = -65536
    const val GREEN: Int = -16711936
    const val BLUE: Int = -16776961
    const val GRAY: Int = -7829368
    const val TRANSPARENT: Int = 0

    fun parseColor(colorString: String): Int = TODO()

    fun rgb(red: Int, green: Int, blue: Int): Int = TODO()

    fun argb(alpha: Int, red: Int, green: Int, blue: Int): Int = TODO()
}

open class Canvas {
    open fun drawColor(color: Int) {
        TODO()
    }

    open fun drawCircle(cx: Float, cy: Float, radius: Float, paint: Paint) {
        TODO()
    }

    open fun drawRect(left: Float, top: Float, right: Float, bottom: Float, paint: Paint) {
        TODO()
    }

    open fun drawText(text: String, x: Float, y: Float, paint: Paint) {
        TODO()
    }
}

open class ColorFilter

object PixelFormat {
    const val OPAQUE: Int = -1
    const val TRANSPARENT: Int = -2
    const val TRANSLUCENT: Int = -3
}

class Bitmap
