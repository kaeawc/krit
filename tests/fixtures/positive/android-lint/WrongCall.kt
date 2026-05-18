package com.example

import android.content.Context
import android.graphics.Canvas
import android.util.AttributeSet
import android.view.View

class CustomViewPositive(context: Context, attrs: AttributeSet?) : View(context, attrs) {
    private val child: View = TODO()

    // Non-override helper that triggers child.onDraw() — wrong, should be child.draw().
    fun forceRedraw(canvas: Canvas) {
        child.onDraw(canvas)
    }
}
