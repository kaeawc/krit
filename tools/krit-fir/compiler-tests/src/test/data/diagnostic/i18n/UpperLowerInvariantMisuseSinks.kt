// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 26
// Only a conversion whose result is shown as user-facing text is reported: an
// argument of a UI text call or the value assigned to a UI text property.
// Everything else is a locale-invariant use and takes no display locale.
package ulimsinks

class Label {
    var text: String = ""
    var tag: String = ""

    fun setTitle(value: String) {
        text = value
    }
}

fun assignedToText(label: Label, userName: String) {
    label.text = <!UpperLowerInvariantMisuse!>userName.uppercase()<!>
}

fun passedToSetter(label: Label, userName: String) {
    label.setTitle(<!UpperLowerInvariantMisuse!>userName.lowercase()<!>)
}

fun templateInSetter(label: Label, userName: String) {
    label.setTitle("Hi ${<!UpperLowerInvariantMisuse!>userName.uppercase()<!>}")
}

fun notAUiProperty(label: Label, userName: String) {
    label.tag = userName.uppercase()
}

fun plainValue(userName: String): String {
    val key = userName.lowercase()
    return key
}

fun comparison(userName: String, other: String): Boolean = userName.lowercase() == other.lowercase()

fun lambdaEndsTheSink(label: Label, names: List<String>) {
    label.setTitle(names.joinToString { it.uppercase() })
}
