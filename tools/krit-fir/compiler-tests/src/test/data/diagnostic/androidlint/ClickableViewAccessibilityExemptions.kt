// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 68
package clickable.exemptions

import android.content.Context
import android.view.MotionEvent
import android.view.View

class AccessibleTouch(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean {
        if (event.action == MotionEvent.ACTION_UP) performClick()
        return true
    }
    override fun performClick(): Boolean = super.performClick()
}

class QualifiedSuper(context: Context) : View(context) {
    override fun performClick(): Boolean = super<View>.performClick()
}

class LabeledSuper(context: Context) : View(context) {
    override fun performClick(): Boolean = super@LabeledSuper.performClick()
}

class CommentExemption(context: Context) : View(context) {
    override fun performClick(): Boolean {
        // Go's raw-source super regex exempts this comment too; preserve it.
        // super.performClick()
        return true
    }
}

class TextExemption(context: Context) : View(context) {
    override fun performClick(): Boolean = "super.performClick()".isNotEmpty()
}

object OtherClicks {
    fun performClick() = Unit
}

class NameOnlyExemption(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean {
        OtherClicks.performClick()
        return true
    }
    override fun performClick(): Boolean = super.performClick()
}

class NestedExemption(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean {
        fun deferredClick() = performClick()
        return true
    }
    override fun performClick(): Boolean = super.performClick()
}

fun attachAccessible(view: View) {
    view.setOnTouchListener { touched, _ -> touched.performClick(); true }
    view.setOnTouchListener { _, _ -> OtherClicks.performClick(); false }
    view.setOnTouchListener(object : View.OnTouchListener {
        override fun onTouch(view: View, event: MotionEvent): Boolean = view.performClick()
    })
    view.setOnTouchListener(null)
}

// Go does not count the quoted call name. FIR resolves the call and preserves this actual click path.
class QuotedClickInvocation(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = `performClick`()
    override fun performClick(): Boolean = super.performClick()
}
