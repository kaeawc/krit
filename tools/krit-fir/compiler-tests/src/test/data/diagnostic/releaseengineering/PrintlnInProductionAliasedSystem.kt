// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses this: it matches the receiver by the spelling `System`, and here
// java.lang.System is imported under another name. FIR reports it.
package test

import java.lang.System as JavaSystem

fun aliasedSystem() {
    <!PrintlnInProduction!>JavaSystem.err.println("aliased System")<!>
}
