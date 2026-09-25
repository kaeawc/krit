// Compiler-test source stubs; never packaged in the production artifact.
package android.animation

abstract class Animator : Cloneable {
    // Java getDuration()/setDuration(long): modeled as a property so the
    // idiomatic `animator.duration = 300` call site resolves.
    abstract var duration: Long

    abstract val isRunning: Boolean

    open fun start() {
        TODO()
    }

    open fun cancel() {
        TODO()
    }

    open fun end() {
        TODO()
    }

    open fun addListener(listener: AnimatorListener) {
        TODO()
    }

    open fun removeAllListeners() {
        TODO()
    }

    interface AnimatorListener {
        fun onAnimationStart(animation: Animator)

        fun onAnimationEnd(animation: Animator)

        fun onAnimationCancel(animation: Animator)

        fun onAnimationRepeat(animation: Animator)
    }
}

open class ValueAnimator : Animator() {
    override var duration: Long
        get() = TODO()
        set(value) = TODO()

    override val isRunning: Boolean
        get() = TODO()

    open val animatedValue: Any?
        get() = TODO()

    fun addUpdateListener(listener: AnimatorUpdateListener) {
        TODO()
    }

    fun interface AnimatorUpdateListener {
        fun onAnimationUpdate(animation: ValueAnimator)
    }

    companion object {
        fun ofFloat(vararg values: Float): ValueAnimator = TODO()

        fun ofInt(vararg values: Int): ValueAnimator = TODO()
    }
}

class ObjectAnimator : ValueAnimator() {
    companion object {
        fun ofFloat(target: Any?, propertyName: String, vararg values: Float): ObjectAnimator = TODO()

        fun ofInt(target: Any?, propertyName: String, vararg values: Int): ObjectAnimator = TODO()
    }
}

class AnimatorSet : Animator() {
    override var duration: Long
        get() = TODO()
        set(value) = TODO()

    override val isRunning: Boolean
        get() = TODO()

    fun playTogether(vararg items: Animator) {
        TODO()
    }

    fun playSequentially(vararg items: Animator) {
        TODO()
    }
}
