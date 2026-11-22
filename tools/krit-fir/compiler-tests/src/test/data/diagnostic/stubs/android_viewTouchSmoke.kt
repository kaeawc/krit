package stubs

import android.view.MotionEvent
import android.view.View

fun bindTouchSmoke(view: View) {
    view.setOnTouchListener { touched, _ ->
        if (touched.isEnabled) touched.performClick()
        true
    }
    view.setOnTouchListener(object : View.OnTouchListener {
        override fun onTouch(touched: View, event: MotionEvent): Boolean = touched.performClick()
    })
    view.setOnTouchListener(null)
}
