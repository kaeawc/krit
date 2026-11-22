// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 27, 28, 33, 42, 43
package clickable.resolution

import android.content.Context
import android.view.MotionEvent
import android.view.View as AndroidBase

typealias TouchBase = AndroidBase
typealias TouchEvent = MotionEvent
typealias TouchResult = Boolean

// Resolution proves View ancestry through aliases that Go may not recognize.
class AliasedTouch(context: Context) : TouchBase(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: TouchEvent): TouchResult = true
    override fun <!ClickableViewAccessibility!>performClick<!>(): TouchResult = false
}

open class GestureBase(context: Context) : AndroidBase(context)
class IndirectTouch(context: Context) : GestureBase(context) {
    override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = false
}

// Go already matches the setter name regardless of receiver syntax.
fun receiverShapes(view: AndroidBase?) {
    view?.setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> false }
    (view ?: return).setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> true }
    with(view) { setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> true } }
}

fun localView(context: Context) {
    class LocalTouch : AndroidBase(context) {
        override fun <!ClickableViewAccessibility!>onTouchEvent<!>(event: MotionEvent): Boolean = true
    }
}

class ListenerSetterOverride(context: Context) : AndroidBase(context) {
    override fun setOnTouchListener(listener: AndroidBase.OnTouchListener?) = super.setOnTouchListener(listener)
}

fun attachSubtype(inherited: IndirectTouch, overridden: ListenerSetterOverride) {
    inherited.setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> true }
    overridden.setOnTouchListener <!ClickableViewAccessibility!>{<!> _, _ -> false }
}

// Go compares raw identifier text and misses quoted names; resolution proves the Android overrides/setter.
class QuotedTouch(context: Context) : android.view.View(context) {
    override fun <!ClickableViewAccessibility!>`onTouchEvent`<!>(event: MotionEvent): Boolean = true
    override fun <!ClickableViewAccessibility!>`performClick`<!>(): Boolean = false
}

fun attachQuoted(view: AndroidBase) {
    view.`setOnTouchListener` <!ClickableViewAccessibility!>{<!> _, _ -> true }
}
