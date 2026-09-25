// Smoke: animator factories, listeners, and the duration property idiom.
package stubs

import android.animation.Animator
import android.animation.AnimatorSet
import android.animation.ObjectAnimator
import android.animation.ValueAnimator
import android.view.View

fun fadeIn(view: View) {
    val fade: ObjectAnimator = ObjectAnimator.ofFloat(view, "alpha", 0f, 1f)
    val scale: ValueAnimator = ValueAnimator.ofFloat(0f, 1f)
    scale.addUpdateListener { animation -> println(animation.animatedValue) }
    val set = AnimatorSet()
    set.playTogether(fade, scale)
    set.addListener(object : Animator.AnimatorListener {
        override fun onAnimationStart(animation: Animator) {}

        override fun onAnimationEnd(animation: Animator) {}

        override fun onAnimationCancel(animation: Animator) {}

        override fun onAnimationRepeat(animation: Animator) {}
    })
    set.duration = 300L
    set.start()
    if (fade.isRunning) fade.cancel()
}
