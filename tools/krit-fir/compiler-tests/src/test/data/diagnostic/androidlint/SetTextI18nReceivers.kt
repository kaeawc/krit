// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positive: TextView receivers Go does not see, each a hardcoded literal
// passed to setText on an android.widget.TextView. Go misses every one of
// them; each group says why.
package test

import android.app.Dialog
import android.content.Context
import android.widget.TextView
import androidx.appcompat.app.AlertDialog

// Deliberate improvement: Go misses a `this.` call inside a scope function on
// a TextView; for a `this` receiver it reads only the enclosing class, which
// is not a TextView. The `this` of the lambda is the TextView.
class ScopedThis(private val label: TextView) {
    fun configure() {
        label.apply { <!SetTextI18n!>this.setText("Applied this")<!> }
        with(label) { <!SetTextI18n!>this.setText("With this")<!> }
        label.run { <!SetTextI18n!>this@run.setText("Run this")<!> }
    }
}

// Deliberate improvement: Go misses a `this@Outer` or `super@Outer` call in an
// inner class; for a `this` or `super` receiver it reads the nearest class
// (Inner), but the receiver is the outer TextView.
open class OuterLabel(context: Context) : TextView(context) {
    inner class Inner {
        fun update() {
            <!SetTextI18n!>this@OuterLabel.setText("Outer this")<!>
            <!SetTextI18n!>super@OuterLabel.setText("Outer super")<!>
        }
    }
}

// Deliberate improvement: Go misses a TextView receiver chain rooted at one of
// its non-View root names (AlertDialog here; also MenuItem, Preference, Tab):
// it skips the chain by that name, but the receiver is a TextView.
fun Dialog.caption(): TextView = TODO()

fun dialogCaption(context: Context) {
    <!SetTextI18n!>AlertDialog.Builder(context).create().caption().setText("Dialog")<!>
}
