// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Negative: a backticked `main` is the function named main, so the call is
// inside main. Go reports this because it compares the raw identifier text,
// "`main`" with the backticks, against "main"; FIR is correct because the
// declaration's name is main.
package test

import kotlin.system.exitProcess

fun `main`() {
    exitProcess(0)
}
