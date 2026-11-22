package fixtures.clickable.positive

import android.content.Context
import android.view.MotionEvent
import android.view.View

class MissingTouchClick(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
}

class MissingTouchInvocation(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = false
    override fun performClick(): Boolean = super.performClick()
}

class MissingClickChain(context: Context) : View(context) {
    override fun performClick(): Boolean = true
}

fun missingTouchListeners(view: View) {
    view.setOnTouchListener { _, _ -> true }
    view.setOnTouchListener(object : View.OnTouchListener {
        override fun onTouch(view: View, event: MotionEvent): Boolean = false
    })
}
