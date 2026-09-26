// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: setText on a TextView with text that is not a string literal
// (a resource id, a variable, a constant, a concatenation, a call result),
// the synthetic `text` property, and setText-like calls on types that are not
// TextViews. Neither Go nor FIR reports any of these.
package test

import android.content.Context
import android.widget.RemoteViews
import android.widget.TextView
import android.widget.Toast
import androidx.core.app.NotificationCompat

const val GREETING = "Hello"

class ResourceScreen(private val title: TextView) {
    fun resources(label: TextView, name: String, count: Int) {
        label.setText(42)
        label.setText(name)
        label.setText(GREETING)
        label.setText("Hello, " + name)
        label.setText("a" + "b")
        label.setText("Items".uppercase())
        label.setText(count.toString())
        label.setText(if (count > 1) "many" else "one")
        title.setText(label.text)
    }

    // The synthetic property is not a setText call; Go does not report it
    // either.
    fun property(label: TextView) {
        label.text = "Assigned"
        title.text = "Assigned"
    }

    fun nonTextViews(context: Context, toast: Toast, remote: RemoteViews) {
        toast.setText("Toast text")
        remote.setTextViewText(1, "label")
        NotificationCompat.Builder(context, "channel")
            .setContentTitle("hi")
            .setContentText("body")
            .setSubText("foo")
    }
}

class Builder {
    fun setText(value: String): Builder = this
}

class PlainHolder {
    fun setText(value: String) {
        setText("recurse")
    }

    fun configure() {
        val b = Builder()
        b.setText("foo")
        Builder().setText("chained")
        this.setText("self")
    }
}

// A top-level function named setText has no receiver.
fun setText(value: String) = value

fun topLevel() {
    setText("top level")
}
