package com.example

import android.graphics.Canvas
import android.graphics.Paint
import android.view.View

class CustomView : View {
    private val paint = Paint()

    override fun onDraw(canvas: Canvas) {
        canvas.drawRect(0f, 0f, 100f, 100f, paint)
    }
}
