// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: a backticked `main` is the function named main, so the call is
// inside main. Go strips the backticks from the declaration's name too.
package test

import kotlin.system.exitProcess

fun `main`() {
    exitProcess(0)
}
