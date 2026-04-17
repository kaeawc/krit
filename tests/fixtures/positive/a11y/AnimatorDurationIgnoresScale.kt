package com.example

import android.animation.ObjectAnimator
import android.animation.ValueAnimator

class AnimatorDurationSample {
    fun ignoresSystemAnimatorScale(target: Any) {
        ValueAnimator.ofFloat(0f, 1f).apply {
            duration = 300
        }.start()

        ObjectAnimator.ofFloat(target, "alpha", 0f, 1f).setDuration(500)
    }
}
