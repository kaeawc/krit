// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 27, 28, 40, 41
// Precision: inside a real android.widget.TextView subclass, `this.` and bare
// setText calls whose receiver is another class's `this`. FIR reports none of
// them. Go reports every one (the go-lines header), each explained below; the
// call does not set the text of a TextView, so none of Go's findings here is a
// true positive.
package test

import android.content.Context
import android.widget.TextView

class Label {
    fun setText(value: String) {}
}

open class TextBuilder {
    open fun setText(value: String) {}
}

// A `this.` or `this@run.` call inside a scope function on a Label sets the
// Label's text. Go reports all three: for a `this` receiver it reads only the
// enclosing class, which is a TextView.
class RealLabelThis(context: Context) : TextView(context) {
    fun configure(label: Label) {
        with(label) { this.setText("With this") }
        label.apply { this.setText("Apply this") }
        label.run { this@run.setText("Run this") }
    }
}

// A bare or `this.` call in an `object : TextBuilder()` member resolves to
// TextBuilder.setText. Go reports both: an object expression is not a class
// declaration, so it reads the enclosing TextView class. (The Runnable object
// in SetTextI18n.kt has no setText of its own, so its bare call does reach the
// TextView, and both report it.)
class RealLabelObject(context: Context) : TextView(context) {
    fun builder(): TextBuilder = object : TextBuilder() {
        fun fill() {
            setText("Builder text")
            this.setText("Builder this")
        }
    }
}
