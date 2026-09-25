// Smoke: custom View/ViewGroup subclasses, inflation, listeners, menus.
package stubs

import android.content.Context
import android.graphics.Canvas
import android.util.AttributeSet
import android.view.ContextThemeWrapper
import android.view.LayoutInflater
import android.view.Menu
import android.view.MenuItem
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup

class SmokeView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0,
) : View(context, attrs, defStyleAttr) {
    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
    }

    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        setMeasuredDimension(10, 10)
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        if (event.action == MotionEvent.ACTION_DOWN) performClick()
        return true
    }

    override fun performClick(): Boolean = super.performClick()

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
    }
}

class SmokeGroup(context: Context, attrs: AttributeSet?) : ViewGroup(context, attrs) {
    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        for (i in 0 until childCount) getChildAt(i)?.layout(l, t, r, b)
    }
}

fun inflateAndBind(parent: ViewGroup, menu: Menu, context: Context): View {
    val themed = ContextThemeWrapper(context, 0)
    val view: View = LayoutInflater.from(themed).inflate(android.R.layout.simple_list_item_1, parent, false)
    // Java getVisibility/setVisibility -> synthetic property; View.VISIBLE is a Java static.
    view.visibility = View.VISIBLE
    if (view.visibility == View.VISIBLE) view.visibility = View.GONE
    view.setOnClickListener { clicked -> clicked.isEnabled = false }
    view.setOnLongClickListener { true }
    view.contentDescription = "row"
    view.post { view.invalidate() }
    val item: MenuItem = menu.add("smoke")
    item.setVisible(true)
    val found: MenuItem? = menu.findItem(item.itemId)
    parent.addView(
        view,
        ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT),
    )
    <!PrintlnInProduction!>println<!>("$found ${view.context} ${view.width} ${view.id}")
    return view
}
