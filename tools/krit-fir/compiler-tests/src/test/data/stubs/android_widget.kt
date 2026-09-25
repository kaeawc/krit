// Compiler-test source stubs; never packaged in the production artifact.
package android.widget

import android.content.Context
import android.graphics.drawable.Drawable
import android.util.AttributeSet
import android.view.View
import android.view.ViewGroup

open class TextView : View {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    open var text: CharSequence
        get() = TODO()
        set(value) = TODO()

    open var hint: CharSequence?
        get() = TODO()
        set(value) = TODO()

    open var error: CharSequence?
        get() = TODO()
        set(value) = TODO()

    open var textSize: Float
        get() = TODO()
        set(value) = TODO()

    fun setText(resid: Int) {
        TODO()
    }

    open fun setTextColor(color: Int) {
        TODO()
    }
}

open class EditText : TextView {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    open fun selectAll() {
        TODO()
    }
}

open class Button : TextView {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)
}

open class ImageView : View {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    open fun setImageResource(resId: Int) {
        TODO()
    }

    open fun setImageDrawable(drawable: Drawable?) {
        TODO()
    }
}

open class ProgressBar : View {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    open var progress: Int
        get() = TODO()
        set(value) = TODO()

    open var max: Int
        get() = TODO()
        set(value) = TODO()

    open var isIndeterminate: Boolean
        get() = TODO()
        set(value) = TODO()
}

open class FrameLayout : ViewGroup {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }
}

open class LinearLayout : ViewGroup {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    open var orientation: Int
        get() = TODO()
        set(value) = TODO()

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }

    companion object {
        const val HORIZONTAL: Int = 0
        const val VERTICAL: Int = 1
    }
}

open class GridLayout : ViewGroup {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    open var columnCount: Int
        get() = TODO()
        set(value) = TODO()

    open var rowCount: Int
        get() = TODO()
        set(value) = TODO()

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }
}

open class ScrollView : FrameLayout {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)
}

open class HorizontalScrollView : FrameLayout {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)
}

@Deprecated("Deprecated in Java")
open class AbsoluteLayout : ViewGroup {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }
}

// Java class with static makeText factories and instance show(); app code
// does not construct or subclass it, so it is an object (see README).
object Toast {
    const val LENGTH_SHORT: Int = 0
    const val LENGTH_LONG: Int = 1

    var duration: Int
        get() = TODO()
        set(value) = TODO()

    fun makeText(context: Context, text: CharSequence, duration: Int): Toast = TODO()

    fun makeText(context: Context, resId: Int, duration: Int): Toast = TODO()

    fun show() {
        TODO()
    }

    fun cancel() {
        TODO()
    }

    fun setText(s: CharSequence) {
        TODO()
    }
}
