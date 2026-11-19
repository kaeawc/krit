// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 21, 27, 33, 39, 47, 53
// detekt reports a check only when the checked type is a subtype of the caught type: only then can a
// further catch clause replace it. Go has no types and reports every check on the caught name.
package test

import java.io.Closeable
import java.io.FileNotFoundException
import java.io.IOException

interface Retryable

fun interfaceCheck(): String {
    try { return "ok" } catch (e: IOException) {
        return if (e is Retryable) "retry" else "other"
    }
}

fun libraryInterfaceCheck(): String {
    try { return "ok" } catch (e: IOException) {
        return if (e !is Closeable) "open" else "closed"
    }
}

fun upcast(): String {
    try { return "ok" } catch (e: IOException) {
        return (e as Any).toString()
    }
}

fun unrelatedGeneric(): String {
    try { return "ok" } catch (e: Exception) {
        return if (e is Comparable<*>) "cmp" else "other"
    }
}

fun subtype(): String {
    try { return "ok" } catch (e: IOException) {
        return if (<!InstanceOfCheckForException!>e is FileNotFoundException<!>) "missing" else "other"
    }
}

typealias IoProblem = IOException

fun sameTypeThroughAlias(): String {
    try { return "ok" } catch (e: IoProblem) {
        return if (<!InstanceOfCheckForException!>e is IOException<!>) "io" else "other"
    }
}

fun nullableTarget(): String {
    try { return "ok" } catch (e: IOException) {
        return (e as FileNotFoundException?).toString()
    }
}
