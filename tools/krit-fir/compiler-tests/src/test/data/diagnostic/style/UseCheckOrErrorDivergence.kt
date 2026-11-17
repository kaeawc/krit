// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 33
// Divergences from Go, which matches the written constructor name only.
package test.usecheckorerrordivergence

import kotlin.IllegalStateException as Boom

// Go misses an import alias of the exception.
fun aliased(cond: Boolean) {
    if (cond) <!UseCheckOrError!>throw Boom("aliased")<!>
}

typealias Broken = IllegalStateException

// Go misses a type alias of the exception.
fun typeAliased(cond: Boolean) {
    if (cond) <!UseCheckOrError!>throw Broken("alias")<!>
}

class Local {
    // Go reports a project class that shares the name; nothing here is an IllegalStateException.
    class IllegalStateException(message: String) : Exception(message)

    fun lookalike(cond: Boolean) {
        if (cond) throw IllegalStateException("local class")
    }
}

// Go reports a function named like the exception.
fun IllegalStateException(message: String): Exception = RuntimeException(message)

fun functionLookalike(cond: Boolean) {
    if (cond) throw IllegalStateException("factory")
}
