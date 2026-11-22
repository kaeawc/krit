package fixtures.clickable.negative

import android.content.Context
import android.view.MotionEvent
import android.view.View

class AccessibleTouchView(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = performClick()
    override fun performClick(): Boolean = super.performClick()
}

class NotTouchView {
    fun onTouchEvent(event: MotionEvent): Boolean = true
}

fun accessibleTouchListeners(view: View) {
    view.setOnTouchListener { touched, _ -> touched.performClick(); true }
    view.setOnTouchListener(object : View.OnTouchListener {
        override fun onTouch(view: View, event: MotionEvent): Boolean = view.performClick()
    })
    view.setOnTouchListener(null)
}
