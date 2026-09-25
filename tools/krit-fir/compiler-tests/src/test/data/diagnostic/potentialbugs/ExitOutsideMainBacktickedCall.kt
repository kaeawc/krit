// RENDER_DIAGNOSTICS_FULL_TEXT
// Positives Go misses: backticks around an identifier do not change the name,
// so each call below resolves to kotlin.system.exitProcess or
// java.lang.System.exit and terminates the process outside main. Go reports
// none of them because it compares the raw identifier text, backticks
// included ("`exitProcess`", "`System`", "`exit`"), against "exitProcess",
// "System" and "exit". This is the same Go behavior as the backticked `main`
// in ExitOutsideMainBacktickedMain.kt, in the other direction.
package test

import kotlin.system.exitProcess

fun backtickedExitProcess() {
    <!ExitOutsideMain!>`exitProcess`(1)<!>
}

fun backtickedQualifiedExitProcess() {
    <!ExitOutsideMain!>kotlin.system.`exitProcess`(3)<!>
}

fun backtickedSystemReceiver() {
    <!ExitOutsideMain!>`System`.exit(1)<!>
}

fun backtickedQualifiedSystemReceiver() {
    <!ExitOutsideMain!>java.lang.`System`.exit(2)<!>
}

fun backtickedExitName() {
    <!ExitOutsideMain!>System.`exit`(1)<!>
}

fun main() {
    `exitProcess`(0)
    kotlin.system.`exitProcess`(0)
    `System`.exit(0)
    java.lang.`System`.exit(0)
    System.`exit`(0)
}
