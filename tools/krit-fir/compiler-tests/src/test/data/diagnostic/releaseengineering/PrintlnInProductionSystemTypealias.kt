// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses this: it matches the receiver by the spelling `System`, and here
// java.lang.System is reached through a typealias. The receiver is still
// System.out, so FIR reports it.
package test

typealias Sys = java.lang.System

fun typealiasedSystem() {
    <!PrintlnInProduction!>Sys.out.println("typealiased System")<!>
}
