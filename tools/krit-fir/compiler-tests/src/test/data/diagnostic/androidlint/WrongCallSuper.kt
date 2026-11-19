// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 35, 39, 43, 50, 56, 62
// Super receivers for WrongCall, as Android Lint reads them: a super call is
// the normal chain-up only in a method of the callback's own name, so it is
// reported anywhere else, qualified (`super<View>`), labeled
// (`super@OuterView`), or plain. Lambdas and local functions are not methods,
// an init block belongs to the constructor, and a property initializer has no
// method. The comments name the ones Go misses and why.
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View

class OuterView(context: Context) : View(context) {
    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        post { super.onDraw(canvas) }
    }

    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        super.onMeasure(widthMeasureSpec, heightMeasureSpec)
        super.<!WrongCall!>onDraw<!>(Canvas())
    }

    override fun onLayout(changed: Boolean, left: Int, top: Int, right: Int, bottom: Int) {
        // A local function is not a method: this is inside onLayout.
        fun again() {
            super.onLayout(changed, left, top, right, bottom)
        }
        again()
    }

    fun plainSuper(canvas: Canvas) {
        super.<!WrongCall!>onDraw<!>(canvas)
    }

    fun qualifiedSuper(canvas: Canvas) {
        super<View>.<!WrongCall!>onDraw<!>(canvas)
    }

    fun labeledSuper(canvas: Canvas) {
        super@OuterView.<!WrongCall!>onMeasure<!>(0, 0)
    }

    // A local function named like the callback is not a method either: the
    // enclosing method is outer.
    fun outer(canvas: Canvas) {
        fun onDraw(c: Canvas) {
            super.<!WrongCall!>onDraw<!>(c)
        }
        onDraw(canvas)
    }

    init {
        super.<!WrongCall!>onMeasure<!>(0, 0)
    }

    val eager: Unit = super.onMeasure(1, 1)

    val lazy: Unit
        get() = super.<!WrongCall!>onMeasure<!>(2, 2)

    inner class Painter {
        // Go misses this: its fallback reads only the nearest class, Painter,
        // which is not a View.
        fun paint(canvas: Canvas) {
            super@OuterView.<!WrongCall!>onDraw<!>(canvas)
        }
    }
}
