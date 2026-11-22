// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15, 20, 25, 29, 34, 36, 42, 53, 57
package clickable.positive

import android.content.Context
import android.view.MotionEvent
import android.view.View
import android.widget.ImageView

class MissingClick(context: Context) : View(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = true
}

class MissingInvocation(context: Context) : ImageView(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = false
    override fun performClick(): Boolean = super.performClick()
}

class InheritedClickIsNotEnough(context: Context) : View(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = performClick()
}

class MissingSuper(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = performClick()
    override fun <!ClickableViewAccessibility!>performClick<!>(): Boolean = true
}

class OnlySuperTouch(context: Context) : View(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = super.onTouchEvent(event)
    override fun performClick(): Boolean = super.performClick()
}

fun attachMissing(view: View) {
    view.setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> true }
    view.setOnTouchListener(object : View.OnTouchListener {
        override fun <!ClickableViewAccessibility!>onTouch<!>(view: View, event: MotionEvent): Boolean = false
    })
}

class SplitAnchor(context: Context) : View(context) {
    override fun
        <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = true
}

// Go parses a next-line lambda separately; K2 proves it is the real View listener argument.
fun attachSplit(view: View?) {
    view?.setOnTouchListener()
        <!ClickableViewAccessibility!>{<!> _, _ -> false }
}

class DocumentationOnly(context: Context) : View(context) {
    /** super.performClick() in KDoc is outside Go's function node, not an exemption. */
    override fun <!ClickableViewAccessibility!>performClick<!>(): Boolean = false
}

class OverloadCountsAsOwn(context: Context) : View(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = true
    fun performClick(extra: Int): Boolean = true
}
