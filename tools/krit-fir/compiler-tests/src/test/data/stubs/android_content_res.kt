// Compiler-test source stubs; never packaged in the production artifact.
package android.content.res

import android.graphics.drawable.Drawable

// App code never constructs Resources (its constructor is deprecated) and
// reads statics like Resources.getSystem(), so it is modeled as an object.
object Resources {
    val configuration: Configuration
        get() = TODO()

    fun getSystem(): Resources = TODO()

    fun getString(id: Int): String = TODO()

    fun getString(id: Int, vararg formatArgs: Any?): String = TODO()

    fun getText(id: Int): CharSequence = TODO()

    fun getQuantityString(id: Int, quantity: Int, vararg formatArgs: Any?): String = TODO()

    fun getStringArray(id: Int): Array<String> = TODO()

    fun getColor(id: Int, theme: Theme?): Int = TODO()

    fun getDimensionPixelSize(id: Int): Int = TODO()

    fun getDrawable(id: Int, theme: Theme?): Drawable = TODO()

    fun getIdentifier(name: String, defType: String?, defPackage: String?): Int = TODO()

    class Theme {
        fun obtainStyledAttributes(attrs: IntArray): TypedArray = TODO()
    }

    class NotFoundException : RuntimeException {
        constructor()

        constructor(name: String?)
    }
}

class TypedArray : AutoCloseable {
    fun getString(index: Int): String? = TODO()

    fun getInt(index: Int, defValue: Int): Int = TODO()

    fun getBoolean(index: Int, defValue: Boolean): Boolean = TODO()

    fun getColor(index: Int, defValue: Int): Int = TODO()

    fun getDimension(index: Int, defValue: Float): Float = TODO()

    fun getResourceId(index: Int, defValue: Int): Int = TODO()

    fun hasValue(index: Int): Boolean = TODO()

    fun recycle() {
        TODO()
    }

    override fun close() {
        TODO()
    }
}

class Configuration {
    @JvmField
    var orientation: Int = 0

    @JvmField
    var uiMode: Int = 0

    @JvmField
    var fontScale: Float = 1f

    companion object {
        const val ORIENTATION_PORTRAIT: Int = 1
        const val ORIENTATION_LANDSCAPE: Int = 2
        const val UI_MODE_NIGHT_MASK: Int = 48
        const val UI_MODE_NIGHT_NO: Int = 16
        const val UI_MODE_NIGHT_YES: Int = 32
        const val UI_MODE_TYPE_NORMAL: Int = 1
    }
}
