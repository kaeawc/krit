// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15
// Only the KDoc directly before a function exempts it. A plain block comment
// there is not KDoc, and the KDoc of the declaration above belongs to that
// declaration, so the call still reports.
package test

/** Prints to the console. */
fun documented() {
    println("documented")
}

/* Not KDoc. */
fun afterPlainComment() {
    <!PrintlnInProduction!>println<!>("after a plain block comment")
}
