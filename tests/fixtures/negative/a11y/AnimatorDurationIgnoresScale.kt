package com.example

import android.animation.ValueAnimator
import android.content.ContentResolver
import android.provider.Settings

class AnimatorDurationSample(private val contentResolver: ContentResolver) {
    fun honorsSystemAnimatorScale() {
        val scale = Settings.Global.getFloat(
            contentResolver,
            Settings.Global.ANIMATOR_DURATION_SCALE,
            1f,
        )

        ValueAnimator.ofFloat(0f, 1f).apply {
            duration = (300 * scale).toLong()
        }.start()
    }
}
