// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 28
// Which argument is the text: extension overloads of setText on a TextView
// whose hardcoded literal is not a plain first positional argument. Go takes
// the first argument that is not named in source; FIR also takes the argument
// bound to the first parameter (the first element of a vararg). A literal in
// either place is hardcoded text on a TextView.
package test

import android.widget.TextView

fun TextView.setText(value: String, bold: Boolean) {
    if (bold) setText(value)
}

fun TextView.setText(emphasis: Boolean, caption: String) {
    if (emphasis) setText(caption)
}

fun TextView.setText(vararg parts: String) {
    setText(parts.joinToString(""))
}

fun textArguments(label: TextView) {
    // The first argument not named in source is the literal; Go reports it.
    <!SetTextI18n!>label.setText(emphasis = true, "NamedFirst")<!>
    // The vararg's first element is the literal; Go reports it.
    <!SetTextI18n!>label.setText("A", "B")<!>
    // Deliberate improvement: Go misses this; every argument is named, so it
    // finds no unnamed argument, but the literal binds the first parameter.
    <!SetTextI18n!>label.setText(value = "NamedOnly", bold = true)<!>
}

// Neither argument path holds a literal; neither Go nor FIR reports these.
fun nonTextArguments(label: TextView, caption: String, parts: Array<String>) {
    label.setText(emphasis = true, caption)
    label.setText(caption, "B")
    label.setText(*parts)
    label.setText(bold = true, value = caption)
}
