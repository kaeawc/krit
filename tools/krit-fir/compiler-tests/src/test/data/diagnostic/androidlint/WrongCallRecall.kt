// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 25, 29, 33, 48, 53, 59, 91, 114, 120, 124, 132, 141
// Recall cases for WrongCall: View receivers reached through framework
// subclasses, anonymous and inner classes, scope functions, casts, and
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

// Go misses the three `this` receivers: source inference does not type a
// scope function's `this`, and outside a View subclass Go has no fallback.
// Both report the forEach `it`, which Go types from the list's element type.
fun measureScoped(chart: PublicChart, charts: List<PublicChart>) {
    chart.apply { <!WrongCall!>this.onMeasure(0, 0)<!> }
    with(chart) { <!WrongCall!>this.onMeasure(0, 0)<!> }
    chart.run { <!WrongCall!>this.onMeasure(0, 0)<!> }
    charts.forEach { <!WrongCall!>it.onMeasure(0, 0)<!> }
}

// Cast receivers: both sides report each of these, since Go types a cast by
// its target type.
fun measureCastChain(chart: PublicChart) {
    <!WrongCall!>(chart as View as PublicChart).onMeasure(0, 0)<!>
}

fun measureCast(any: Any) {
    <!WrongCall!>(any as PublicChart).onMeasure(0, 0)<!>
}

// Go misses one of these: tree-sitter parses a statement that starts with `(`
// right after another statement as a call suffix on the line above, so the
// two lines become one expression and Go reports it once. Kotlin ends the
// first statement at the newline, so both are View onMeasure calls.
fun measureCastsAdjacent(chart: PublicChart, any: Any) {
    <!WrongCall!>(chart as View as PublicChart).onMeasure(0, 0)<!>
    <!WrongCall!>(any as PublicChart).onMeasure(0, 0)<!>
}

// Both sides report the parenthesized receiver split over lines, on its first
// line. Go misses the `?.let { it }` chain: it does not type the `let`
// result, and outside a View subclass it has no fallback. FIR reports on the
// first line of the call expression.
fun measureSplit(v: PublicChart, m: PublicChart?) {
    <!WrongCall!>(<!>
        v
    ).onMeasure(0, 0)
    <!WrongCall!>m<!>
        ?.let { it }
        ?.onMeasure(0, 0)
}

// Go misses this: it does not type a local holding an object expression. The
// object is a View that widens onDraw to public.
fun drawAnonymousLocal(context: Context, canvas: Canvas) {
    val obj = object : View(context) {
        public override fun onDraw(canvas: Canvas) {
        }
    }
    <!WrongCall!>obj.onDraw(canvas)<!>
}

interface Measurable

// Go misses these: the receiver is a type parameter with an intersection
// bound, which source inference does not follow.
fun <T> measureBoth(chart: T) where T : PublicChart, T : Measurable {
    <!WrongCall!>chart.onMeasure(0, 0)<!>
}

class BoundHost(context: Context) : View(context) {
    fun <T> measureBoth(chart: T) where T : PublicChart, T : Measurable {
        <!WrongCall!>chart.onMeasure(0, 0)<!>
    }
}
