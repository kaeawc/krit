// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive Go misses: with the star static import `java.lang.System.*`, the
// bare `exit(1)` resolves to java.lang.System.exit and terminates the process
// outside main. Go reports System.exit only with a receiver spelled `System`
// or `java.lang.System`, so it reports nothing here.
package test

import java.lang.System.*

fun starImportedExit() {
    <!ExitOutsideMain!>exit(1)<!>
}

fun main() {
    exit(0)
}
