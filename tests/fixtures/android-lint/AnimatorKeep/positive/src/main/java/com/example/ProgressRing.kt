package com.example

import android.animation.ObjectAnimator
import android.content.Context
import android.view.View

class ProgressRing(context: Context) : View(context) {
    // R8 may remove or rename this setter: ObjectAnimator reaches it only by reflection.
    var sweep: Float = 0f

    fun animateTo(target: Float) {
        ObjectAnimator.ofFloat(this, "sweep", sweep, target).start()
    }
}
