// fir-parity: skip onDraw is protected in android.view.View, so K2 rejects child.onDraw() on a View-typed receiver from a subclass (INVISIBLE_REFERENCE) and this fixture never compiles cleanly; Go stays authoritative for it, and the compiling FIR positives (a receiver typed as the calling View subclass, or a subclass that widens the callback to public) are covered by compiler-tests data
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
