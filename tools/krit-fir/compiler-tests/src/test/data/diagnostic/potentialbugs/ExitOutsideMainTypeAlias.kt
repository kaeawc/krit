// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive Go misses: `Sys` is a type alias for java.lang.System, so
// `Sys.exit(1)` resolves to java.lang.System.exit and terminates the process
// outside main. Go reports System.exit only when the receiver is spelled
// `System` or `java.lang.System`, so it reports nothing here.
package test

typealias Sys = java.lang.System

fun typeAliasedSystem() {
    <!ExitOutsideMain!>Sys.exit(1)<!>
}

fun main() {
    Sys.exit(0)
}
