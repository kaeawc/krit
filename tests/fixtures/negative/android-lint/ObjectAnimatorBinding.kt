package test

import android.animation.ObjectAnimator as Animator
import android.view.View

class Meter {
    var colour: Float = 0f
    fun setProgressFraction(value: Float) {}
}

class AnimatorFactory {
    companion object {
        fun ofFloat(target: Meter, propertyName: String, value: Float) {}
    }
}

fun cleanViewProperties(view: View) {
    Animator.ofFloat(view, "translationX", 1f)
    Animator.ofFloat(
        view,
        "alpha",
        1f,
    )
    Animator.ofFloat(view, "rotation", 1f)
}

fun cleanCustomProperties(meter: Meter) {
    Animator.ofFloat(meter, "progressFraction", 1f)
    Animator.ofFloat(meter, "colour", 1f)
}

fun ignoredInputs(view: View, propertyName: String, meter: Meter) {
    Animator.ofFloat(view, propertyName, 1f)
    Animator.ofFloat(missingTarget, "translatoinX", 1f)
    AnimatorFactory.ofFloat(meter, "missing", 1f)
    val sample = "ObjectAnimator.ofFloat(view, \"translatoinX\", 1f)"
    // ObjectAnimator.ofFloat(view, "translatoinX", 1f)
}
