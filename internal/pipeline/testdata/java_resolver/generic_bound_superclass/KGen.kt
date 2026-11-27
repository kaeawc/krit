package app

import android.content.Context
import android.graphics.Canvas
import j.Gen

class KGen(c: Context) : Gen<String>(c) {
    override fun onDraw(canvas: Canvas) {
        val w = canvas.width
    }
}
