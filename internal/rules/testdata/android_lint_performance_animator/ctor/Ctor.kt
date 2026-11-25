package p

import android.animation.ObjectAnimator
import android.content.Context
import android.view.View

class Meter(context: Context, var level: Float, val fixed: Float) : View(context) {
    fun animate() {
        ObjectAnimator.ofFloat(this, "level", 0f, 1f)
        ObjectAnimator.ofFloat(this, "fixed", 0f, 1f)
    }
}
