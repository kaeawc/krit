// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 25, 29, 33, 50, 54, 59, 65, 97
// Recall cases for WrongCall: View receivers reached through framework
// subclasses, anonymous and inner classes, qualified super receivers, and
// receiver expressions whose type only resolution knows. Every call below
// runs a View's own onDraw / onMeasure / onLayout on an explicit receiver
// outside an override, so every one is reported; the comments name the ones
// Go misses and why.
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View
import android.widget.FrameLayout
import android.widget.TextView

class BadgeText(context: Context) : TextView(context) {
    fun redraw(other: BadgeText, canvas: Canvas) {
        <!WrongCall!>other.onDraw(canvas)<!>
    }
}

class CardFrame(context: Context) : FrameLayout(context) {
    fun relayout(other: CardFrame) {
        <!WrongCall!>other.onLayout(true, 0, 0, 1, 1)<!>
    }

    fun first(frames: List<CardFrame>) {
        <!WrongCall!>frames.first().onMeasure(0, 0)<!>
    }

    fun indexed(frames: List<CardFrame?>) {
        <!WrongCall!>frames[0]?.onMeasure(0, 0)<!>
    }
}

class OuterView(context: Context) : View(context) {
    inner class Painter {
        // Go misses this: it cannot type `this@OuterView`, and its fallback
        // looks only at the nearest enclosing class, Painter, which is not a
        // View.
        fun paint(canvas: Canvas) {
            <!WrongCall!>this@OuterView.onDraw(canvas)<!>
        }
    }

    // Go skips only a receiver spelled `super`, so both report a qualified
    // `super<View>` from a helper that is not the onDraw override.
    fun qualifiedSuper(canvas: Canvas) {
        <!WrongCall!>super<View>.onDraw(canvas)<!>
    }

    fun parenthesized(other: OuterView?, canvas: Canvas) {
        <!WrongCall!>(other!!).onDraw(canvas)<!>
    }

    fun smartCast(candidate: Any, canvas: Canvas) {
        if (candidate is OuterView) {
            <!WrongCall!>candidate.onDraw(canvas)<!>
        }
    }

    fun local(canvas: Canvas) {
        val self = this
        <!WrongCall!>self.onDraw(canvas)<!>
    }
}

// Go misses this: its fallback needs a class_declaration or
// object_declaration that extends View, and an object expression is neither.
fun anonymous(context: Context): View = object : View(context) {
    fun paint(canvas: Canvas) {
        <!WrongCall!>this.onDraw(canvas)<!>
    }
}

// A View subclass named like a builder root Go treats as a non-View
// (`Tab`, as in TabLayout.Tab). Go skips any receiver chain that starts with
// that name; this receiver is a View, so FIR reports it.
class Tab(context: Context) : View(context) {
    public override fun onDraw(canvas: Canvas) {
    }
}

fun drawTab(context: Context, canvas: Canvas) {
    <!WrongCall!>Tab(context).onDraw(canvas)<!>
}

typealias Chart = PublicChart

open class PublicChart(context: Context) : View(context) {
    public override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
    }
}

fun measureAlias(chart: Chart) {
    <!WrongCall!>chart.onMeasure(0, 0)<!>
}

// Go misses this: source inference does not type the result of
// `listOf(chart).first()`, and outside a View subclass it has no fallback.
fun measureGeneric(chart: PublicChart) {
    val chosen = listOf(chart).first()
    <!WrongCall!>chosen.onMeasure(0, 0)<!>
}

// Go misses this: the receiver is a type parameter bounded by a View
// subclass, which source inference does not follow.
fun <T : PublicChart> measureBound(chart: T) {
    <!WrongCall!>chart.onMeasure(0, 0)<!>
}
