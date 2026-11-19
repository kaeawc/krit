// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 19, 23, 27, 31, 36, 40, 46, 52, 65, 69
// Positives for WrongCall: a View's onDraw / onMeasure / onLayout called
// directly on an explicit receiver, reported on the callee name. The
// callbacks are protected, so K2 only accepts a receiver typed as the calling
// View subclass (or a subclass that widens the callback to public).
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View

class ChartView(context: Context) : View(context) {
    fun redrawSelf(canvas: Canvas) {
        this.<!WrongCall!>onDraw<!>(canvas)
    }

    fun redrawOther(other: ChartView, canvas: Canvas) {
        other.<!WrongCall!>onDraw<!>(canvas)
    }

    fun remeasure(other: ChartView) {
        other.<!WrongCall!>onMeasure<!>(0, 0)
    }

    fun relayout(other: ChartView) {
        other.<!WrongCall!>onLayout<!>(true, 0, 0, 10, 10)
    }

    fun maybe(other: ChartView?, canvas: Canvas) {
        other?.<!WrongCall!>onDraw<!>(canvas)
    }

    fun split(other: ChartView, canvas: Canvas) {
        other
            .<!WrongCall!>onDraw<!>(canvas)
    }

    fun inLambda(canvas: Canvas) {
        post { this.<!WrongCall!>onDraw<!>(canvas) }
    }

    // A local function is the nearest named function: not an override.
    override fun onAttachedToWindow() {
        fun refresh(canvas: Canvas) {
            this.<!WrongCall!>onDraw<!>(canvas)
        }
        refresh(Canvas())
    }

    init {
        this.<!WrongCall!>onMeasure<!>(0, 0)
    }
}

// A subclass that widens onDraw to public can be called from anywhere.
open class PublicDrawView(context: Context) : View(context) {
    public override fun onDraw(canvas: Canvas) {
    }
}

class PublicDrawChild(context: Context) : PublicDrawView(context)

fun drawPublic(view: PublicDrawView, canvas: Canvas) {
    view.<!WrongCall!>onDraw<!>(canvas)
}

fun drawPublicChild(view: PublicDrawChild, canvas: Canvas) {
    view.<!WrongCall!>onDraw<!>(canvas)
}
