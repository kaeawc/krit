// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 13, 14, 16, 28
// Divergence (precision): imports of project declarations named
// MODE_WORLD_READABLE whose parent is a Kotlin enum, a companion object or a
// class (an enum entry, a companion constant, a nested class), and their uses.
// Go reports each imported name, the enum entry's declaration and the bare use
// by the identifier's text (not the aliased uses H and K); none of them is the
// platform's world-readable file mode, so FIR reports none of them. Resolving
// these import parents does not throw.
package test

import test.Holder.Companion.MODE_WORLD_READABLE as H
import test.Kls.MODE_WORLD_READABLE as K
import test.Mode.MODE_WORLD_READABLE

enum class Mode { MODE_WORLD_READABLE }

class Holder {
    companion object {
        const val MODE_WORLD_READABLE = 0
    }
}

class Kls {
    class MODE_WORLD_READABLE
}

fun imported() = listOf(MODE_WORLD_READABLE, H, K())
