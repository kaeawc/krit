// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 18, 24, 30, 41, 46, 55
// Implicit receivers and overrides for WrongCall, as Android Lint reads them:
// a bare callback call in a View subclass, a scope function's `this`, and a
// receiver other than super inside an override of the same name are all
// reported. Only a super receiver is exempt in a same-named override. The
// comments name the ones Go misses and why.
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View
import android.widget.FrameLayout

class DialView(context: Context) : View(context) {
    fun bare(canvas: Canvas) {
        <!WrongCall!>onDraw<!>(canvas)
        <!WrongCall!>onMeasure<!>(0, 0)
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val other = DialView(context)
        other.<!WrongCall!>onDraw<!>(canvas)
    }

    // Recursion through the implicit receiver is reported as well.
    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        super.onMeasure(widthMeasureSpec, heightMeasureSpec)
        <!WrongCall!>onMeasure<!>(1, 2)
    }

    // Go misses these: it skips a bare call inside a lambda, which may have a
    // receiver of its own.
    override fun onLayout(changed: Boolean, left: Int, top: Int, right: Int, bottom: Int) {
        post { <!WrongCall!>onMeasure<!>(0, 0) }
        with(this) { <!WrongCall!>onMeasure<!>(0, 0) }
    }

    init {
        <!WrongCall!>onMeasure<!>(0, 0)
    }

    val measured: Int
        get() {
            <!WrongCall!>onMeasure<!>(0, 0)
            return 0
        }

    // An overload declared in a View subclass is a member of a View subclass.
    fun onDraw(pass: Int) {
    }

    fun overload() {
        <!WrongCall!>onDraw<!>(1)
    }

    // Go misses this: from an inner class it reads only the nearest class.
    inner class Ticker {
        fun tick(canvas: Canvas) {
            <!WrongCall!>onDraw<!>(canvas)
        }
    }
}

// Go misses this: the file declares a top-level onLayout extension (see
// ExtensionHost), and Go, which cannot tell the overloads apart, skips a bare
// call when a function outside a class has the callback's name.
class FrameHost(context: Context) : FrameLayout(context) {
    fun relayout() {
        <!WrongCall!>onLayout<!>(true, 0, 0, 1, 1)
    }
}

open class OpenDial(context: Context) : View(context) {
    public override fun onDraw(canvas: Canvas) {
    }
}

// Go misses these: a scope function's `this` outside a View subclass.
fun drawScoped(dial: OpenDial, canvas: Canvas) {
    dial.apply { <!WrongCall!>onDraw<!>(canvas) }
    with(dial) { <!WrongCall!>onDraw<!>(canvas) }
}

class Painter {
    fun onDraw(canvas: Canvas) {
    }
}

// A lambda's receiver that is not a View, inside a View subclass.
class PainterHost(context: Context) : View(context) {
    private val painter = Painter()

    fun paint(canvas: Canvas) {
        with(painter) { onDraw(canvas) }
        val local = object {
            fun onMeasure(width: Int, height: Int) {
            }

            fun go() {
                onMeasure(1, 1)
            }
        }
        local.go()
    }
}

// A top-level extension named like a callback is not a View member.
fun View.onLayout(tag: String) {
}

class ExtensionHost(context: Context) : View(context) {
    fun f() {
        onLayout("x")
    }
}
