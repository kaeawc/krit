// Compiler-test source stubs; never packaged in the production artifact.
package android.view

import android.content.Context
import android.content.ContextWrapper
import android.graphics.Canvas
import android.util.AttributeSet

open class ContextThemeWrapper : ContextWrapper {
    constructor() : super(null)

    constructor(base: Context?, themeResId: Int) : super(base)
}

open class View {
    constructor(context: Context)

    constructor(context: Context, attrs: AttributeSet?)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int, defStyleRes: Int)

    val context: Context
        get() = TODO()

    // Java getter/setter pairs that app code reads/writes as properties.
    open var visibility: Int
        get() = TODO()
        set(value) = TODO()

    open var isEnabled: Boolean
        get() = TODO()
        set(value) = TODO()

    open var isClickable: Boolean
        get() = TODO()
        set(value) = TODO()

    open var alpha: Float
        get() = TODO()
        set(value) = TODO()

    open var id: Int
        get() = TODO()
        set(value) = TODO()

    open var tag: Any?
        get() = TODO()
        set(value) = TODO()

    open var contentDescription: CharSequence?
        get() = TODO()
        set(value) = TODO()

    val width: Int
        get() = TODO()

    val height: Int
        get() = TODO()

    val parent: ViewParent?
        get() = TODO()

    open fun setOnClickListener(l: OnClickListener?) {
        TODO()
    }

    open fun setOnLongClickListener(l: OnLongClickListener?) {
        TODO()
    }

    open fun performClick(): Boolean = TODO()

    fun <T : View> findViewById(id: Int): T = TODO()

    open fun post(action: Runnable): Boolean = TODO()

    open fun postDelayed(action: Runnable, delayMillis: Long): Boolean = TODO()

    open fun invalidate() {
        TODO()
    }

    open fun requestLayout() {
        TODO()
    }

    open fun layout(l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }

    protected open fun onDraw(canvas: Canvas) {
        TODO()
    }

    protected open fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        TODO()
    }

    protected open fun onLayout(changed: Boolean, left: Int, top: Int, right: Int, bottom: Int) {
        TODO()
    }

    protected fun setMeasuredDimension(measuredWidth: Int, measuredHeight: Int) {
        TODO()
    }

    open fun onTouchEvent(event: MotionEvent): Boolean = TODO()

    protected open fun onAttachedToWindow() {
        TODO()
    }

    protected open fun onDetachedFromWindow() {
        TODO()
    }

    fun interface OnClickListener {
        fun onClick(v: View)
    }

    fun interface OnLongClickListener {
        fun onLongClick(v: View): Boolean
    }

    companion object {
        const val VISIBLE: Int = 0
        const val INVISIBLE: Int = 4
        const val GONE: Int = 8

        fun generateViewId(): Int = TODO()
    }
}

interface ViewParent

abstract class ViewGroup : View, ViewParent {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    constructor(
        context: Context,
        attrs: AttributeSet?,
        defStyleAttr: Int,
        defStyleRes: Int,
    ) : super(context, attrs, defStyleAttr, defStyleRes)

    val childCount: Int
        get() = TODO()

    abstract override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int)

    open fun addView(child: View) {
        TODO()
    }

    open fun addView(child: View, params: LayoutParams) {
        TODO()
    }

    open fun removeView(view: View) {
        TODO()
    }

    open fun removeAllViews() {
        TODO()
    }

    fun getChildAt(index: Int): View? = TODO()

    open class LayoutParams(width: Int, height: Int) {
        @JvmField
        var width: Int = width

        @JvmField
        var height: Int = height

        companion object {
            const val MATCH_PARENT: Int = -1
            const val WRAP_CONTENT: Int = -2
        }
    }

    open class MarginLayoutParams(width: Int, height: Int) : LayoutParams(width, height)
}

// Java abstract class obtained via the static LayoutInflater.from(Context).
object LayoutInflater {
    fun from(context: Context): LayoutInflater = TODO()

    fun inflate(resource: Int, root: ViewGroup?): View = TODO()

    fun inflate(resource: Int, root: ViewGroup?, attachToRoot: Boolean): View = TODO()
}

interface Menu {
    fun add(title: CharSequence?): MenuItem

    fun add(groupId: Int, itemId: Int, order: Int, title: CharSequence?): MenuItem

    fun findItem(id: Int): MenuItem?

    fun size(): Int

    fun clear()
}

interface MenuItem {
    val itemId: Int

    val title: CharSequence?

    val isVisible: Boolean

    fun setVisible(visible: Boolean): MenuItem

    fun setTitle(title: CharSequence?): MenuItem
}

open class MenuInflater(context: Context) {
    open fun inflate(menuRes: Int, menu: Menu) {
        TODO()
    }
}

object MotionEvent {
    const val ACTION_DOWN: Int = 0
    const val ACTION_UP: Int = 1
    const val ACTION_MOVE: Int = 2
    const val ACTION_CANCEL: Int = 3

    val action: Int
        get() = TODO()

    val x: Float
        get() = TODO()

    val y: Float
        get() = TODO()

    fun obtain(downTime: Long, eventTime: Long, action: Int, x: Float, y: Float, metaState: Int): MotionEvent = TODO()
}
