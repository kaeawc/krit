// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 15, 32, 42
// Early-exit guards: `if (<x is not a CharArray>) <exit>` leaves x a CharArray
// in the statements after it.
package test

// Under `||` the exit is taken whenever x is not a CharArray.
fun orGuard(value: Any, flag: Boolean): String {
    if (value !is CharArray || flag) return ""
    return <!CharArrayToStringCall!>value.toString()<!>
}

fun orGuardRight(value: Any, flag: Boolean): String {
    if (flag || value !is CharArray) return ""
    return <!CharArrayToStringCall!>value.toString()<!>
}

// Go misses this: its early-exit test accepts only return, throw, TODO, and
// error, but `continue` also skips the rest of the loop body.
fun continueGuard(items: List<Any>) {
    for (item in items) {
        if (item !is CharArray) continue
        println(<!CharArrayToStringCall!>item.toString()<!>)
    }
}

// Go reports these; the guard does not always exit when the value is not a
// CharArray, so the value is not proven to be one and FIR drops them.
// Under `&&`, a false flag skips the exit.
fun andGuard(value: Any, flag: Boolean): String {
    if (value !is CharArray && flag) return ""
    return value.toString()
}

// Go finds the nested return anywhere in the guard's body, but with a false
// flag the body falls through.
fun nestedExit(value: Any, flag: Boolean): String {
    if (value !is CharArray) {
        if (flag) return ""
        println()
    }
    return value.toString()
}
