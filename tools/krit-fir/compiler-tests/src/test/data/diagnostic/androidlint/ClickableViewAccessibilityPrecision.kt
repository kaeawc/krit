// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 21, 29, 39, 49, 50
package clickable.precision

import android.content.Context
import android.view.MotionEvent

open class View {
    open fun onTouchEvent(event: MotionEvent): Boolean = true
    open fun performClick(): Boolean = true
    fun setOnTouchListener(listener: (View, MotionEvent) -> Boolean) = Unit
}

// Go recognizes the simple name View; resolved project methods are not Android APIs.
class ProjectView : View() {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
    override fun performClick(): Boolean = false
}

fun attachProject(view: View) {
    view.setOnTouchListener { _, _ -> true }
}

// Go accepts any setter name, even on an unrelated receiver.
class TouchRegistry {
    fun setOnTouchListener(listener: (Any, Any) -> Boolean) = Unit
}
fun attachRegistry(registry: TouchRegistry) {
    registry.setOnTouchListener { _, _ -> true }
}

// Go walks onTouch by name; FIR requires the real OnTouchListener override.
interface ProjectListener {
    fun onTouch(view: View, event: MotionEvent): Boolean
}
fun TouchRegistry.setOnTouchListener(listener: ProjectListener) = Unit
fun attachProjectObject(registry: TouchRegistry) {
    registry.setOnTouchListener(object : ProjectListener {
        override fun onTouch(view: View, event: MotionEvent): Boolean = true
    })
}

// Go's override-name check also matches a project interface overload on a real View.
interface ExtraTouch {
    fun onTouchEvent(event: String): Boolean
    fun performClick(extra: Int): Boolean
}
class OverloadedView(context: Context) : android.view.View(context), ExtraTouch {
    override fun onTouchEvent(event: String): Boolean = true
    override fun performClick(extra: Int): Boolean = true
}

// Go is silent for a parenthesized SAM lambda. Deliberately preserve that false negative.
fun parenthesizedListener(view: android.view.View) {
    view.setOnTouchListener({ _, _ -> true })
}

// A stored listener's implementation is not inspected by Go or this file-local checker.
class StoredListener : android.view.View.OnTouchListener {
    override fun onTouch(view: android.view.View, event: MotionEvent): Boolean = true
}
fun attachStored(view: android.view.View, listener: StoredListener) {
    view.setOnTouchListener(listener)
}
