package app

import android.graphics.Canvas
import android.view.View
import j.JBox

class KBox : JBox<View, String>() {
    fun onDraw(canvas: Canvas) {
        val w = canvas.width
    }
}
