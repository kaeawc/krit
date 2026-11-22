// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package clickable.anonymousviews

import android.content.Context
import android.view.MotionEvent
import android.view.View

interface TouchContract {
    fun onTouchEvent(event: MotionEvent): Boolean
    fun performClick(): Boolean
}

class ListenerHost(context: Context) : View(context) {
    val listener = object : TouchContract {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
        override fun performClick(): Boolean = true
    }
}

class CompanionHost(context: Context) : View(context) {
    companion object : TouchContract {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
        override fun performClick(): Boolean = true
    }
}

// Conservative FN in both engines: Lint checks anonymous Views, but these overrides
// must not be attributed to the enclosing host. Anonymous class checks stay silent.
class AnonymousTouchHost(context: Context) : View(context) {
    val child = object : View(context) {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
    }
}

class AnonymousClickHost(context: Context) : View(context) {
    val child = object : View(context) {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
        override fun performClick(): Boolean = true
    }
}
