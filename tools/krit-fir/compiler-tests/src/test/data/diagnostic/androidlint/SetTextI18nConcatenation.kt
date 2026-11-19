// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): hardcoded text that Android Lint's SetTextDetector finds
// and Go does not. Lint reads a literal among the operands of a `+`
// concatenation and in the receiver of a qualified call, and assigning the
// synthetic `text` property of a TextView calls TextView.setText. Go reads only
// a bare literal or template passed to a `setText(...)` call.
package test.concatenation

import android.widget.Button
import android.widget.TextView

class Labels(private val title: TextView, private val button: Button) {
    fun concatenated(label: TextView, name: String) {
        <!SetTextI18n!>label.setText("Hello, " + name)<!>
        <!SetTextI18n!>label.setText(name + " items")<!>
        <!SetTextI18n!>label.setText("Items".uppercase())<!>
        <!SetTextI18n!>label.setText("%d items".format(3))<!>
    }

    fun assigned(label: TextView, name: String) {
        <!SetTextI18n!>label.text = "Assigned"<!>
        <!SetTextI18n!>title.text = "Hello, $name"<!>
        <!SetTextI18n!>button.text = "Click " + name<!>
        label.apply { <!SetTextI18n!>text = "In scope"<!> }
    }
}

class Header(context: android.content.Context) : TextView(context) {
    fun reset() {
        <!SetTextI18n!>text = "Default"<!>
    }
}

// A `text` property of another type is not TextView.setText.
class Note {
    var text: CharSequence = ""
}

fun note(n: Note) {
    n.text = "Plain note"
}
