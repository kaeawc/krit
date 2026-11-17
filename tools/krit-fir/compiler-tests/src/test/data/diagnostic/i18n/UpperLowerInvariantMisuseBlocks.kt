// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A block inside a UI text argument passes on only its last statement, the
// value of an `if`, `when` or `try` branch. Go stops at any block, so it
// misses the branch values; FIR reports them and skips the other statements,
// whose results are not shown.
package ulimblocks

class Label {
    fun setText(value: String) {}
}

fun ifValue(label: Label, show: Boolean, userName: String) {
    label.setText(if (show) { <!UpperLowerInvariantMisuse!>userName.uppercase()<!> } else "")
}

fun whenValue(label: Label, kind: Int, userName: String) {
    label.setText(
        when (kind) {
            1 -> {
                val trimmed = userName.trim()
                <!UpperLowerInvariantMisuse!>trimmed.lowercase()<!>
            }
            else -> ""
        },
    )
}

fun tryValue(label: Label, userName: String) {
    label.setText(try { <!UpperLowerInvariantMisuse!>userName.uppercase()<!> } catch (e: IllegalStateException) { "" })
}

// Not the block's value: the conversion is logged, not shown.
fun earlierStatement(label: Label, show: Boolean, userName: String) {
    label.setText(if (show) { println(userName.uppercase()); "shown" } else "")
}

fun earlierWhenStatement(label: Label, kind: Int, userName: String) {
    label.setText(
        when (kind) {
            1 -> {
                val key = userName.lowercase()
                key.length.toString()
            }
            else -> ""
        },
    )
}
