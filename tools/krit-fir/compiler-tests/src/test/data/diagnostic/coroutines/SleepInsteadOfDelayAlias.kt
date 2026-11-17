// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go matches the spelling Thread.sleep; the alias resolves to java.lang.Thread.sleep.
package test

import java.lang.Thread as JavaThread

suspend fun aliased() {
    JavaThread.<!SleepInsteadOfDelay!>sleep<!>(10)
}
