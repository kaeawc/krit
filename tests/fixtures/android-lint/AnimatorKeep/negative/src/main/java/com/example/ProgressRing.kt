package com.example

import android.animation.ObjectAnimator
import android.content.Context
import android.view.View
import androidx.annotation.Keep

class ProgressRing(context: Context) : View(context) {
    @set:Keep
    var sweep: Float = 0f

    @Keep
    fun setGlow(value: Float) {}

    fun animateTo(target: Float) {
        ObjectAnimator.ofFloat(this, "sweep", sweep, target).start()
        ObjectAnimator.ofFloat(this, "glow", 0f, 1f).start()
        // alpha is View's own setter, which is compiled and kept by the platform.
        ObjectAnimator.ofFloat(this, "alpha", 0f, 1f).start()
    }
}
