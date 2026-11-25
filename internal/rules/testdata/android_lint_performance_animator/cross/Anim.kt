package p

import android.animation.ArgbEvaluator
import android.animation.ObjectAnimator
import android.animation.PropertyValuesHolder
import android.content.Context
import android.view.View

class Anim(private val context: Context) {
    fun run(g: Gauge, k: KeptGauge, j: JGauge, v: View) {
        ObjectAnimator.ofFloat(g, "level", 0f, 1f).start()
        ObjectAnimator.ofFloat(g, "kept", 0f, 1f)
        ObjectAnimator.ofFloat(g, "radius", 0f, 1f)
        ObjectAnimator.ofFloat(g, "safe", 0f, 1f)
        ObjectAnimator.ofInt(g, "count", 0, 1)
        ObjectAnimator.ofFloat(g, "count", 0f, 1f)
        ObjectAnimator.ofFloat(k, "level", 0f, 1f)
        ObjectAnimator.ofFloat(j, "level", 0f, 1f)
        ObjectAnimator.ofFloat(j, "static", 0f, 1f)
        ObjectAnimator.ofFloat(v, "alpha", 0f, 1f)
        ObjectAnimator.ofFloat(g, "alpha", 0f, 1f)
        ObjectAnimator.ofObject(g, "count", ArgbEvaluator(), 0, 1)
        val holder = PropertyValuesHolder.ofFloat("radius", 0f, 1f)
        ObjectAnimator.ofPropertyValuesHolder(g, holder, PropertyValuesHolder.ofInt("count", 0, 1))
        val name = "level"
        ObjectAnimator.ofFloat(g, name, 0f, 1f)
        ObjectAnimator.ofFloat(Gauge(context), "radius", 0f, 1f)
    }
}
