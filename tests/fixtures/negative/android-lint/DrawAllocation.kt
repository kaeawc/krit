package com.example

import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Rect
import android.view.View

class CustomView : View {
    private val paint = Paint()
    private val rect = Rect(0, 0, 100, 100)

    override fun onDraw(canvas: Canvas) {
        // new Paint() — comment mentioning a constructor must not trigger
        val s = "Paint(leftover = \"{\")"
        canvas.drawRect(rect, paint)
    }
}

class Sibling {
    fun renderSomething() {
        val p = Paint()
    }
}

class NotAView {
    override fun onDraw(canvas: Canvas) {
        doSomething()
    }

    private fun doSomething() {}
}
