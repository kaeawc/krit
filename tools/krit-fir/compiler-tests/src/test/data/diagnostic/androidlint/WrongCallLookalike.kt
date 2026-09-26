// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 35, 36, 42, 50, 52, 53, 64, 69, 82, 87, 99, 125, 131, 132, 133, 134
// Lookalikes for WrongCall: project extensions named onDraw on
// android.view.View, function-typed properties and interface methods named
// like the callbacks, and non-View receivers called from inside a View
// subclass. None of them calls a View's onDraw / onMeasure / onLayout, so the
// message ("should probably call draw/measure/layout instead") is not true of
// them and FIR does not report them. The comments name the ones Go reports and
// why. The one marker is a judgment call kept as Go has it (see
// OverloadView).
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View

// A project extension with the callback's exact signature. View.onDraw is
// protected, so K2 resolves every `v.onDraw(canvas)` below to this visible
// extension, not to the invisible member: the call runs this function.
fun View.onDraw(canvas: Canvas) {
    invalidate()
}

// An extension overload with a different parameter list.
fun View.onDraw(pass: Int) {
}

open class EView(context: Context) : View(context)

// Go reports each call: the receiver is a View (or a View subclass), and Go
// does not tell the extension apart from View's own onDraw. Each call runs a
// project extension, not the View callback.
fun extensions(v: View, e: EView, canvas: Canvas) {
    v.onDraw(canvas)
    v.onDraw(1)
    e.onDraw(2)
}

class Other(context: Context) : View(context) {
    // Go reports this: the receiver is a View. It runs the extension.
    fun f(v: View, canvas: Canvas) {
        v.onDraw(canvas)
    }
}

class SubE(context: Context) : EView(context) {
    // Go reports all three: each receiver is a View subclass. Each call passes
    // an Int, so it runs the `onDraw(pass: Int)` extension.
    fun f(other: EView) {
        other.onDraw(1)
        val self = this
        self.onDraw(2)
        this.onDraw(3)
    }
}

// A function-typed property named like a callback on a View subclass.
// Go reports both calls because the receiver is a View subtype; each call
// invokes the lambda property, not View.onLayout.
class FView(context: Context) : View(context) {
    val onLayout: (Int) -> Unit = {}

    fun f(other: FView) {
        other.onLayout(1)
    }
}

fun invokeLayout(v: FView) {
    v.onLayout(2)
}

// An interface default method inherited by a View subclass. Go reports both
// calls because the receiver is a View subtype; each runs Sketcher.onDraw,
// whose owner is not a View.
interface Sketcher {
    fun onDraw(pass: Int) {
    }
}

class PView(context: Context) : View(context), Sketcher {
    fun f(other: PView) {
        other.onDraw(1)
    }
}

fun sketch(v: PView) {
    v.onDraw(1)
}

// Judgment call, kept as Go has it: an onDraw overload declared directly in a
// View subclass is not a View callback either, but its owner is a View, and
// FIR reports it as Go does. Only a callee whose owner is not a View (the
// extensions, the lambda property, and Sketcher.onDraw above) is dropped.
class OverloadView(context: Context) : View(context) {
    fun onDraw(canvas: Canvas, extra: Int) {
    }

    fun f(other: OverloadView, canvas: Canvas) {
        <!WrongCall!>other.onDraw(canvas, 1)<!>
    }
}

class Painter {
    val onDraw: (Canvas) -> Unit = {}

    fun onMeasure(width: Int, height: Int) {
    }
}

fun makePainter(): Painter = Painter()

class HostView(context: Context) : View(context) {
    private val painters = mapOf(1 to Painter())
    private val painter = Painter()

    fun paint(canvas: Canvas) {
        makePainter().onMeasure(0, 0)
        painters.getValue(1).onMeasure(0, 0)
        painter.onMeasure(0, 0)
        painter.onDraw(canvas)
    }

    // Go reports this: the receiver is a View. It runs the extension.
    fun extensionInView(other: View) {
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
