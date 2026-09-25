// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Console output Go misses and FIR reports. Each call resolves to
// kotlin.io.println / print or to print / println on System.out / System.err.
package test

import java.lang.System.out

// Go misses every bare call in this file: it skips any file that declares a
// function named println, even an unrelated member that is not in scope at
// the call. The calls below still resolve to kotlin.io.println.
class Unrelated {
    fun println(value: Int) {
        require(value > 0)
    }
}

fun recall() {
    <!PrintlnInProduction!>println("still kotlin.io.println")<!>
    // Go only reports the exact System.out.<name> / System.err.<name> chain.
    <!PrintlnInProduction!>kotlin.io.println("package qualified")<!>
    <!PrintlnInProduction!>java.lang.System.out.println("fully qualified System")<!>
    <!PrintlnInProduction!>out.println("statically imported out")<!>
    <!PrintlnInProduction!>(System.out).println("parenthesized receiver")<!>
}
