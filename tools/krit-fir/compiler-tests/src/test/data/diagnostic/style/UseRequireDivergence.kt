// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 33
// Divergences from Go, which matches the written constructor name only.
package test.userequiredivergence

import kotlin.IllegalArgumentException as Bad

// Go misses an import alias of the exception.
fun aliased(cond: Boolean) {
    if (cond) <!UseRequire!>throw Bad("aliased")<!>
}

typealias Wrong = IllegalArgumentException

// Go misses a type alias of the exception.
fun typeAliased(cond: Boolean) {
    if (cond) <!UseRequire!>throw Wrong("alias")<!>
}

class Local {
    // Go reports a project class that shares the name; nothing here is an IllegalArgumentException.
    class IllegalArgumentException(message: String) : Exception(message)

    fun lookalike(cond: Boolean) {
        if (cond) throw IllegalArgumentException("local class")
    }
}

// Go reports a function named like the exception.
fun IllegalArgumentException(message: String): Exception = RuntimeException(message)

fun functionLookalike(cond: Boolean) {
    if (cond) throw IllegalArgumentException("factory")
}
