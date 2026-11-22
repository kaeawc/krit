// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 29, 34, 35
package clickable.abstractviews

import android.content.Context
import android.view.MotionEvent
import android.view.View

typealias TouchBase = View

abstract class AbstractTouch(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
}

abstract class AbstractClick(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
    override fun performClick(): Boolean = true
}

abstract class AbstractAlias(context: Context) : TouchBase(context) {
    abstract override fun onTouchEvent(event: MotionEvent): Boolean
    override fun performClick(): Boolean = true
}

abstract class AbstractListeners(context: Context) : View(context) {
    fun attach(view: View) {
        view.setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> true }
        view.setOnTouchListener(object : View.OnTouchListener {
            override fun <!ClickableViewAccessibility!>onTouch<!>(view: View, event: MotionEvent): Boolean = true
        })
    }

    class ConcreteNested(context: Context) : View(context) {
        override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = true
        override fun <!ClickableViewAccessibility!>performClick<!>(): Boolean = true
    }
}

sealed class SealedTouch(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
}

sealed class SealedClick(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
    override fun performClick(): Boolean = true
}
