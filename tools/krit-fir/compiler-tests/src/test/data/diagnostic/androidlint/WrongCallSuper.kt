// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 28
// Super receivers for WrongCall. The rule allows a `super` receiver anywhere:
// a super call deliberately runs the superclass implementation, which
// draw / measure / layout cannot do, so the message ("should probably call
// draw/measure/layout instead") is not true of it. FIR skips every super
// receiver, qualified (`super<View>`) or labeled (`super@OuterView`), as well
// as the plain spelling. The comments name the ones Go reports and why.
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View

class OuterView(context: Context) : View(context) {
    fun plainSuper(canvas: Canvas) {
        super.onDraw(canvas)
    }

    // Go reports this: it skips only a receiver spelled exactly `super`.
    fun qualifiedSuper(canvas: Canvas) {
        super<View>.onDraw(canvas)
    }

    // Go reports this for the same reason, then falls back to "the enclosing
    // class extends View".
    fun labeledSuper(canvas: Canvas) {
        super@OuterView.onMeasure(0, 0)
    }

    inner class Painter {
        // Neither side reports this: Go's fallback reads only the nearest
        // class, Painter, which is not a View.
        fun paint(canvas: Canvas) {
            super@OuterView.onDraw(canvas)
        }
    }
}
