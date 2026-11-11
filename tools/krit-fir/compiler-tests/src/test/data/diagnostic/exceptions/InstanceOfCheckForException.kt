// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 22, 31, 40
package test

import java.io.IOException

fun isCheck(): String {
    try {
        return "ok"
    } catch (e: Exception) {
        if (<!InstanceOfCheckForException!>e is IOException<!>) {
            return "io"
        }
        return "other"
    }
}

fun notIsCheck(): String {
    try {
        return "ok"
    } catch (e: Exception) {
        if (<!InstanceOfCheckForException!>e !is IOException<!>) return "not io"
        return "io"
    }
}

fun unsafeCast(): String {
    try {
        return "ok"
    } catch (e: Exception) {
        val io = <!InstanceOfCheckForException!>e as IOException<!>
        return io.toString()
    }
}

fun insideLambda(): String {
    try {
        return "ok"
    } catch (e: Exception) {
        return listOf(1).map { if (<!InstanceOfCheckForException!>e is IOException<!>) "io" else "x" }.toString()
    }
}
