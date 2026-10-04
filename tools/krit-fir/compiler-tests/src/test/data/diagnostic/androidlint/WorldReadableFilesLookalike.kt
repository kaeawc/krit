// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 28, 30, 32, 34, 36x2, 38, 42, 48
// Divergence (precision): project declarations that merely share the name
// MODE_WORLD_READABLE. Go reports every identifier spelled MODE_WORLD_READABLE
// except a property or variable name, so it reports the enum entry
// declaration, the uses of an object constant, a companion constant and an
// enum entry, a parameter's declaration and use, a named argument's label,
// and the uses of a local and of a member property that shadow Context's
// constant (both hold MODE_PRIVATE). None of them is the platform's
// world-readable file mode, so FIR reports none of them. A project constant
// initialized from Context.MODE_WORLD_READABLE still reports on its
// initializer (WorldReadableFiles.kt, `const val WORLD`).
package test

import android.app.Activity
import android.content.Context

object Modes {
    const val MODE_WORLD_READABLE = 0
}

class Holder {
    companion object {
        const val MODE_WORLD_READABLE = 0
    }
}

enum class Mode { MODE_WORLD_READABLE }

fun objectConstant() = Modes.MODE_WORLD_READABLE

fun companionConstant() = Holder.MODE_WORLD_READABLE

fun enumEntry() = Mode.MODE_WORLD_READABLE

fun parameter(MODE_WORLD_READABLE: Int) = MODE_WORLD_READABLE

fun namedArgument() = parameter(MODE_WORLD_READABLE = 1)

fun local(context: Context) {
    val MODE_WORLD_READABLE = Context.MODE_PRIVATE
    context.getSharedPreferences("data", MODE_WORLD_READABLE)
}

class Shadowing : Activity() {
    val MODE_WORLD_READABLE = MODE_PRIVATE

    fun prefs() = getSharedPreferences("data", MODE_WORLD_READABLE)
}
