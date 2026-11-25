package p

import android.animation.ObjectAnimator
import android.content.Context
import android.view.View
import androidx.annotation.Keep

class Dial(context: Context) : View(context) {
    var level: Float = 0f
    @Keep
    var kept: Float = 0f
    @set:Keep
    var setKept: Float = 0f
    fun setRadius(r: Float) {}
    @Keep fun setSafe(x: Float) {}

    fun animate() {
        ObjectAnimator.ofFloat(this, "radius", 0f, 1f)
        ObjectAnimator.ofFloat(this, "safe", 0f, 1f)
        ObjectAnimator.ofFloat(this, "level", 0f, 1f)
        ObjectAnimator.ofFloat(this, "kept", 0f, 1f)
        ObjectAnimator.ofFloat(this, "setKept", 0f, 1f)
    }
}
