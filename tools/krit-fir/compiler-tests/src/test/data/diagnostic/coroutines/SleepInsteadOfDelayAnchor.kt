// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 14
// Go and detekt report the call on the callee name, so a chain split across
// lines keeps the finding on `.sleep`, and a reference argument on the reference.
package test

suspend fun chained() {
    Thread
        .<!SleepInsteadOfDelay!>sleep<!>(10)
}

suspend fun referenceArgument(items: List<Long>) {
    items.forEach(
        <!SleepInsteadOfDelay!>Thread::sleep<!>
    )
}
