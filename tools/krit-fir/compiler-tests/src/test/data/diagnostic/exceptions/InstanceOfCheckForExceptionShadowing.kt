// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 22, 52
package test

import java.io.IOException

class Holder(val e: Any)

// A lambda parameter that reuses the caught name is not the caught exception (Go reports lines 14 and 22 by spelling).
fun lambdaParameter(items: List<Any>): String {
    try {
        return "ok"
    } catch (e: Exception) {
        return items.map { e -> e is IOException }.toString()
    }
}

fun localShadow(): String {
    try {
        return "ok"
    } catch (e: Exception) {
        val inner = { e: Any -> e as String }
        return inner("x")
    }
}

fun memberSameName(h: Holder): String {
    try {
        return "ok"
    } catch (e: Exception) {
        return if (h.e is IOException) "io" else "other"
    }
}

// The parenthesised operand is still the caught exception (Go misses it: its first child is not the identifier).
fun parenthesised(): String {
    try {
        return "ok"
    } catch (e: Exception) {
        return if (<!InstanceOfCheckForException!>(e) is IOException<!>) "io" else "other"
    }
}

// An outer catch parameter read inside a nested catch is still a caught exception.
fun nestedCatch(): String {
    try {
        return "ok"
    } catch (outer: Exception) {
        try {
            return "inner"
        } catch (inner: IllegalStateException) {
            return if (<!InstanceOfCheckForException!>outer is IOException<!>) "io" else "other"
        }
    }
}
