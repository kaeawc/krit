package com.example

import android.graphics.Canvas
import android.graphics.Paint
import android.view.View

class CustomView : View {
    override fun onDraw(canvas: Canvas) {
        val paint = Paint()
        val items = mutableListOf<Int>()
        canvas.drawRect(0f, 0f, 100f, 100f, paint)
    }
}
