// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 21, 22, 29, 30, 31, 35, 36, 37, 41, 48, 58x2, 67, 77, 95, 101, 102, 103, 110, 125, 136, 142, 172, 173, 194, 206, 208
// Positive: a string literal or template passed as the text of setText on an
// android.widget.TextView or a subtype (Button, EditText, project
// subclasses). Findings sit on the first line of the call expression. Go
// reports only some of these (the go-lines header); each group it misses is
// marked below.
package test

import android.content.Context
import android.util.AttributeSet
import android.widget.Button
import android.widget.EditText
import android.widget.TextView

class HardcodedScreen(private val title: TextView, private val submit: Button) {
    private val field: EditText = TODO()

    fun onProperties() {
        <!SetTextI18n!>title.setText("Hello, world")<!>
        <!SetTextI18n!>submit.setText("Click me")<!>
        <!SetTextI18n!>field.setText("")<!>
        // Deliberate improvement: Go misses this; it does not type the
        // `this.title` receiver, and its name fallback does not match `title`.
        <!SetTextI18n!>this.title.setText("Qualified")<!>
    }

    fun onParameters(label: TextView, ok: Button, input: EditText) {
        <!SetTextI18n!>label.setText("Label")<!>
        <!SetTextI18n!>ok.setText("OK")<!>
        <!SetTextI18n!>input.setText("Name")<!>
    }

    fun templates(label: TextView, count: Int) {
        <!SetTextI18n!>label.setText("Count: $count")<!>
        <!SetTextI18n!>label.setText("${count} items")<!>
        <!SetTextI18n!>label.setText("""Raw text""")<!>
    }

    fun nullableReceivers(label: TextView?) {
        <!SetTextI18n!>label?.setText("Maybe")<!>
        // Deliberate improvement: Go misses this; it does not type a `!!`
        // receiver.
        <!SetTextI18n!>label!!.setText("Surely")<!>
    }

    fun multiLine(label: TextView, holder: HardcodedScreen?) {
        <!SetTextI18n!>label<!>
            .setText("Split")
        // Deliberate improvement: Go misses this; it does not type the
        // safe-call chain through the private `title` property.
        <!SetTextI18n!>holder<!>
            ?.title
            ?.setText("Split safe")
    }

    fun twoOnOneLine(label: TextView) {
        <!SetTextI18n!>label.setText("One")<!>; <!SetTextI18n!>label.setText("Two")<!>
    }

    // Go types the local cast to Button. Deliberate improvement: Go misses
    // the local inferred from findViewById and the call chained on it; it
    // does not type either receiver, and its name fallback does not match
    // `view`.
    fun inferredLocals(root: android.view.View, any: Any) {
        val widget = any as Button
        <!SetTextI18n!>widget.setText("Cast")<!>
        val view = root.findViewById<TextView>(1)
        <!SetTextI18n!>view.setText("Found")<!>
        <!SetTextI18n!>root.findViewById<TextView>(2).setText("Chained")<!>
    }

    // Go types the lambda parameter `it`. Deliberate improvement: Go misses
    // the bare calls on the implicit receiver of a scope function; it reads
    // only the enclosing class, which is not a TextView.
    fun scopeFunctions(label: TextView, labels: List<TextView>) {
        labels.forEach { <!SetTextI18n!>it.setText("Each")<!> }
        label.apply { <!SetTextI18n!>setText("Applied")<!> }
        with(label) { <!SetTextI18n!>setText("With")<!> }
        label.run {
            <!SetTextI18n!>setText("Run")<!>
        }
    }

    // Deliberate improvement: Go needs the literal as a bare first argument,
    // so it misses a parenthesized, annotated, or labeled one; each is still
    // hardcoded text.
    fun wrappedLiterals(label: TextView) {
        <!SetTextI18n!>label.setText(("Parenthesized"))<!>
        <!SetTextI18n!>label.setText(@Suppress("UNUSED") "Annotated")<!>
        <!SetTextI18n!>label.setText(text@ "Labeled")<!>
    }

    // A property initializer outside any function.
    val initial = <!SetTextI18n!>title.setText("Initial")<!>
}

// A TextView subclass: a bare, `this.`, or `super.` call sets its own text.
open class CustomLabel(context: Context, attrs: AttributeSet?) : TextView(context, attrs) {
    fun reset() {
        <!SetTextI18n!>setText("Default label")<!>
        <!SetTextI18n!>this.setText("This label")<!>
        <!SetTextI18n!>super.setText("Super label")<!>
    }

    // An anonymous object is not a class declaration; Go reads the enclosing
    // CustomLabel and reports, like FIR.
    fun later(): Runnable = object : Runnable {
        override fun run() {
            <!SetTextI18n!>setText("Later")<!>
        }
    }

    // Deliberate improvement: Go reads the nearest enclosing class (Inner),
    // which is not a TextView, and misses the outer receiver.
    inner class Inner {
        fun update() {
            <!SetTextI18n!>setText("Inner")<!>
        }
    }

    // A local function's body is still in CustomLabel, the class Go reads.
    fun local() {
        fun refresh() {
            <!SetTextI18n!>setText("Local")<!>
        }
        refresh()
    }
}

// An indirect subclass. Go follows the class hierarchy for a bare call and
// for a typed parameter. Deliberate improvement: Go misses `this.setText`; for
// a `this` or `super` receiver it matches only the direct supertypes by name.
class FancyLabel(context: Context) : CustomLabel(context, null) {
    fun fancy() {
        <!SetTextI18n!>setText("Fancy")<!>
        <!SetTextI18n!>this.setText("Fancy this")<!>
    }
}

fun onFancy(fancy: FancyLabel) {
    <!SetTextI18n!>fancy.setText("Fancy param")<!>
}

// Deliberate improvement: Go misses an extension receiver, implicit or
// `this`, because the call is not inside a class.
fun TextView.showPlaceholder() {
    <!SetTextI18n!>setText("Placeholder")<!>
    <!SetTextI18n!>this.setText("This placeholder")<!>
}

// Deliberate improvement: a TextView-bounded type parameter is a TextView.
fun <T : TextView> T.greet(): T {
    <!SetTextI18n!>setText("Hi")<!>
    return this
}

// A project overload on a TextView subclass and an extension overload on
// TextView: the receiver is a TextView and the text is hardcoded, so both
// Go and FIR report them.
class SuffixLabel(context: Context) : TextView(context) {
    fun setText(value: String, suffix: String) {
        setText(value + suffix)
    }
}

fun TextView.setText(value: String, bold: Boolean) {
    if (bold) setText(value)
}

fun overloads(suffix: SuffixLabel, label: TextView) {
    <!SetTextI18n!>suffix.setText("Suffixed", "!")<!>
    <!SetTextI18n!>label.setText("Bold", true)<!>
}

// An anonymous TextView subclass reports its own bare call and a call through
// a local. Deliberate improvement: Go misses both; an object expression is
// not a class declaration, and it does not type the local.
fun anonymousLabel(context: Context): TextView {
    val label = object : TextView(context) {
        fun reset() {
            <!SetTextI18n!>setText("Anonymous")<!>
        }
    }
    <!SetTextI18n!>label.setText("Anonymous local")<!>
    label.reset()
    return label
}

// A local TextView subclass: its bare call sets its own text.
fun localLabel(context: Context): TextView {
    class LocalLabel : TextView(context) {
        fun reset() {
            <!SetTextI18n!>setText("Local class")<!>
        }
    }
    return LocalLabel()
}

typealias Caption = TextView

// A smart cast and a typealias; Go reports both. Deliberate improvement: Go
// misses the parenthesized safe-cast receiver, which it does not type.
fun resolvedShapes(view: android.view.View, caption: Caption) {
    if (view is TextView) {
        <!SetTextI18n!>view.setText("Smart cast")<!>
    }
    <!SetTextI18n!>caption.setText("Aliased")<!>
    <!SetTextI18n!>(view as? Button)?.setText("Safe cast")<!>
}
