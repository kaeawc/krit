// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 22x2, 24, 27, 30
// Divergence (precision): more project declarations that merely share the
// name MODE_WORLD_READABLE. Go reports every identifier spelled
// MODE_WORLD_READABLE except a property or variable name, so it reports a
// top-level function's declaration and call, a nested class's constructor
// call, the uses of a lambda parameter and of a for-loop variable, and the use
// of an extension property on Context that returns MODE_PRIVATE. None of them
// is the platform's world-readable file mode, so FIR reports none of them.
package test

import android.content.Context

fun MODE_WORLD_READABLE() = 1

class Kls {
    class MODE_WORLD_READABLE
}

val Context.MODE_WORLD_READABLE: Int get() = Context.MODE_PRIVATE

fun call() = MODE_WORLD_READABLE() + Kls.MODE_WORLD_READABLE().hashCode()

fun lambdaParameter() = listOf(1).map { MODE_WORLD_READABLE -> MODE_WORLD_READABLE + 1 }

fun loopVariable(xs: List<Int>) {
    for (MODE_WORLD_READABLE in xs) println(MODE_WORLD_READABLE)
}

fun extension(context: Context) = context.getSharedPreferences("data", context.MODE_WORLD_READABLE)
