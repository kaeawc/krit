// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: setText on a TextView with text that holds no string literal with
// a word run (a resource id, a variable, a constant, a concatenation of short
// literals, a call result), the synthetic `text` property set from such text,
// and setText-like calls on types that are not TextViews. Neither Go nor FIR
// reports any of these.
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
        label.setText("a" + "b")
        label.setText(count.toString())
        label.setText(if (count > 1) "many" else "one")
        title.setText(label.text)
    }

    fun property(label: TextView, name: String) {
        label.text = name
        title.text = "a" + "b"
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
