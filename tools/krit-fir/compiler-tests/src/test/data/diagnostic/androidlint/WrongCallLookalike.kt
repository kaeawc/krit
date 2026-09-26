// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 27, 65, 66, 67, 68
// Lookalikes for WrongCall: a project class named View, a project extension
// named onDraw on android.view.View, a function-typed onDraw property, and
// non-View receivers called from inside a View subclass. None of them calls a
// View's onDraw / onMeasure / onLayout, so the message ("should probably call
// draw/measure/layout instead") is not true of any of them and FIR reports
// none. The comments name the ones Go reports and why.
package test

import android.content.Context
import android.graphics.Canvas

class View {
    fun onDraw(canvas: Canvas) {
    }

    fun onLayout(changed: Boolean) {
    }
}

// Go reports both calls: it proves a View receiver by the simple name `View`,
// and this View is the project class above, not android.view.View.
class Screen {
    fun paint(view: View, canvas: Canvas) {
        view.onDraw(canvas)
        view.onLayout(true)
    }
}

fun android.view.View.onDraw(pass: Int) {
}

fun extension(view: android.view.View) {
    view.onDraw(1)
}

class Painter {
    val onDraw: (Canvas) -> Unit = {}

    fun onMeasure(width: Int, height: Int) {
    }
}

fun makePainter(): Painter = Painter()

class HostView(context: Context) : android.view.View(context) {
    private val painters = mapOf(1 to Painter())
    private val painter = Painter()

    fun paint(canvas: Canvas) {
        makePainter().onMeasure(0, 0)
        painters.getValue(1).onMeasure(0, 0)
        painter.onMeasure(0, 0)
        painter.onDraw(canvas)
    }

    fun extensionInView(other: android.view.View) {
        other.onDraw(1)
    }

    // Go reports each of these: it cannot type the receiver, so it falls back
    // to "the enclosing class extends View". Each call runs Painter.onMeasure.
    fun untyped(maybe: Painter?) {
        maybe!!.onMeasure(0, 0)
        painter.let { it }.onMeasure(0, 0)
        run { painter }.onMeasure(0, 0)
        painters[1]?.onMeasure(0, 0)
    }
}
