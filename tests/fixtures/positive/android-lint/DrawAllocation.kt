package com.example

import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Rect
import android.view.View

class CustomView : View {
    override fun onDraw(canvas: Canvas) {
        val paint = Paint()
        val rect = Rect(0, 0, 100, 100)
        canvas.drawRect(rect, paint)
    }

    override fun draw(canvas: Canvas) {
        val bounds = Rect(0, 0, width, height)
        super.draw(canvas)
    }
}
