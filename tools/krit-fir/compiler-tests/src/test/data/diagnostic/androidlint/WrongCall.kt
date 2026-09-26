// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 19, 23, 27, 31, 35, 40, 46, 52, 65, 69
// Positives for WrongCall: a View's onDraw / onMeasure / onLayout called
// directly on an explicit receiver outside an override. The callbacks are
// protected, so K2 only accepts a receiver typed as the calling View subclass
// (or a subclass that widens the callback to public).
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View

class ChartView(context: Context) : View(context) {
    fun redrawSelf(canvas: Canvas) {
        <!WrongCall!>this.onDraw(canvas)<!>
    }

    fun redrawOther(other: ChartView, canvas: Canvas) {
        <!WrongCall!>other.onDraw(canvas)<!>
    }

    fun remeasure(other: ChartView) {
        <!WrongCall!>other.onMeasure(0, 0)<!>
    }

    fun relayout(other: ChartView) {
        <!WrongCall!>other.onLayout(true, 0, 0, 10, 10)<!>
    }

    fun maybe(other: ChartView?, canvas: Canvas) {
        <!WrongCall!>other?.onDraw(canvas)<!>
    }

    fun split(other: ChartView, canvas: Canvas) {
        <!WrongCall!>other<!>
            .onDraw(canvas)
    }

    fun inLambda(canvas: Canvas) {
        post { <!WrongCall!>this.onDraw(canvas)<!> }
    }

    // A local function is the nearest named function: not an override.
    override fun onAttachedToWindow() {
        fun refresh(canvas: Canvas) {
            <!WrongCall!>this.onDraw(canvas)<!>
        }
        refresh(Canvas())
    }

    init {
        <!WrongCall!>this.onMeasure(0, 0)<!>
    }
}

// A subclass that widens onDraw to public can be called from anywhere.
open class PublicDrawView(context: Context) : View(context) {
    public override fun onDraw(canvas: Canvas) {
    }
}

class PublicDrawChild(context: Context) : PublicDrawView(context)

fun drawPublic(view: PublicDrawView, canvas: Canvas) {
    <!WrongCall!>view.onDraw(canvas)<!>
}

fun drawPublicChild(view: PublicDrawChild, canvas: Canvas) {
    <!WrongCall!>view.onDraw(canvas)<!>
}
