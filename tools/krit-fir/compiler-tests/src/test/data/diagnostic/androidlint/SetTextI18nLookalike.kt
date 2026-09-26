// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 30, 39, 49, 50, 61, 63, 68
// Precision: setText calls with a hardcoded literal whose receiver is not an
// android.widget.TextView. FIR reports none of them. Go reports some (the
// go-lines header), each explained below; the call does not set the text of a
// TextView, so none of Go's findings here is a true positive.
package test

import android.content.Context
import android.widget.TextView as AndroidTextView

// Project classes that are only named like Android widgets.
class TextView {
    fun setText(value: String) {}
}

open class Button {
    open fun setText(value: String) {}
}

class Label {
    fun setText(value: String) {}
}

class Row(val titleText: Label, val button: Label)

// Go reports `text.setText`: it types `text` as a class named TextView and
// accepts the simple name. The other three Go types as a non-TextView too.
fun lookalikeReceivers(text: TextView, button: Button, row: Row) {
    text.setText("Lookalike")
    button.setText("Lookalike button")
    row.titleText.setText("Row title")
    row.button.setText("Row button")
}

// Go reports the first call: it does not type `row.titleText` here and falls
// back to the name, which ends in "text". The receiver is a Label.
fun lambdaReceivers(rows: List<Row>) {
    rows.forEach { row -> row.titleText.setText("Row title") }
    rows.forEach { it.button.setText("Row button") }
}

// Go reports both calls: for a bare or `this` call it accepts a direct
// supertype named Button, and this Button is the project class above.
class FakeButton : Button() {
    override fun setText(value: String) {}

    fun reset() {
        setText("Fake")
        this.setText("Fake this")
    }
}

// Inside a real TextView subclass, a bare call inside `with(label)` or
// `label.apply` sets the Label's text, and a local function named setText
// shadows the member. Go reports all three: for a bare call it reads only the
// enclosing class, which is a TextView.
class RealLabel(context: Context) : AndroidTextView(context) {
    fun configure(label: Label) {
        with(label) {
            setText("Label text")
        }
        label.apply { setText("Label apply") }
    }

    fun shadowed() {
        fun setText(value: String) = value
        setText("Local function")
    }
}
