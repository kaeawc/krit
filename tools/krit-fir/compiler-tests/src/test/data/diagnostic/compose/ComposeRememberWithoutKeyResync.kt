// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33, 43, 45, 46
// A parameter written back into the remembered object outside the calculation keeps it current: an assignment in
// an also/apply block chained on the remember call, or a set*/update*/sync* call on the remembered variable. Reads
// of the remembered value, calls that assign nothing, and updates of another variable are not re-syncs.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

fun use(x: Any?) {}

class DragState(val scope: Any, var index: Int) {
    fun updateIndex(value: Int) {
        index = value
    }
}

class Holder(var label: String)

class Palette {
    fun copy(): Palette = Palette()
    fun updateColors(other: Palette) {}
}

class Formatter(val locale: String) {
    fun format(value: String): String = value
}

// Only scope is reported: index is written back after every composition.
@Composable
fun Resynced(scope: Any, index: Int, label: String, colors: Palette) {
    val target = <!ComposeRememberWithoutKey!>remember<!> { DragState(scope, index) }.also { it.index = index }
    val applied = remember { Holder(label) }.apply { this.label = label }
    val palette = remember { colors.copy() }
    palette.updateColors(colors)
    use(target)
    use(applied)
}

@Composable
fun NotResynced(locale: String, index: Int, other: Int) {
    val formatter = <!ComposeRememberWithoutKey!>remember<!> { Formatter(locale) }
    use(formatter.format(locale))
    val holder = <!ComposeRememberWithoutKey!>remember<!> { DragState(1, index) }.also { use(it); use(index) }
    val copy = <!ComposeRememberWithoutKey!>remember<!> { DragState(2, other) }
    holder.updateIndex(other)
    use(copy)
}
