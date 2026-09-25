// Compiler-test source stubs; never packaged in the production artifact.
package androidx.core.view

import android.view.View

object ViewCompat {
    fun setOnApplyWindowInsetsListener(v: View, listener: OnApplyWindowInsetsListener?) {
        TODO()
    }

    fun setElevation(view: View, elevation: Float) {
        TODO()
    }

    fun requestApplyInsets(view: View) {
        TODO()
    }
}

fun interface OnApplyWindowInsetsListener {
    fun onApplyWindowInsets(v: View, insets: WindowInsetsCompat): WindowInsetsCompat
}

class WindowInsetsCompat {
    fun isVisible(typeMask: Int): Boolean = TODO()

    object Type {
        fun systemBars(): Int = TODO()

        fun ime(): Int = TODO()

        fun navigationBars(): Int = TODO()

        fun statusBars(): Int = TODO()
    }

    companion object {
        val CONSUMED: WindowInsetsCompat
            get() = TODO()
    }
}

var View.isVisible: Boolean
    get() = TODO()
    set(value) = TODO()

var View.isGone: Boolean
    get() = TODO()
    set(value) = TODO()

var View.isInvisible: Boolean
    get() = TODO()
    set(value) = TODO()
