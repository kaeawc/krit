// RENDER_DIAGNOSTICS_FULL_TEXT
// Positives Go misses: the calls below resolve to kotlin.system.exitProcess or
// java.lang.System.exit through an import alias or a static import, so they do
// terminate the process outside main. Go matches only the literal spellings
// `exitProcess(...)` and `System.exit(...)` / `java.lang.System.exit(...)` and
// reports none of them.
package test

import java.lang.System as JavaSystem
import java.lang.System.exit
import java.lang.System.exit as quit
import kotlin.system.exitProcess as die

fun aliasedExitProcess() {
    <!ExitOutsideMain!>die(1)<!>
}

fun staticImportExit() {
    <!ExitOutsideMain!>exit(1)<!>
}

fun aliasedStaticImportExit() {
    <!ExitOutsideMain!>quit(1)<!>
}

fun aliasedSystemClass() {
    <!ExitOutsideMain!>JavaSystem.exit(1)<!>
}

fun main() {
    die(0)
    exit(0)
    quit(0)
    JavaSystem.exit(0)
}
