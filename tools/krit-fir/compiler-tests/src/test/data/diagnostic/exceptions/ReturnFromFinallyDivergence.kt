// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 24
package test

fun fetch(): String = "data"

// Go skips a labeled return only when the label equals the name of the lambda's callee, so a custom label on
// an inner lambda is reported, although that return only leaves the lambda.
fun customLabelOnInnerLambda(items: List<String>): Int {
    try {
        fetch()
    } finally {
        items.forEach inner@{ if (it.isEmpty()) return@inner }
    }
    return 0
}

// A custom outer label targets the enclosing lambda, not the inline lambda inside finally.
// Both engines report this return; resolution must not skip every custom label indiscriminately.
fun labelOnOuterLambda(): Int = run outer@{
    try {
        fetch()
    } finally {
        run { <!ReturnFromFinally!>return@outer 1<!> }
    }
    0
}

// Go skips this return because its label matches the inner lambda callee name, while FIR resolves it
// to the enclosing run lambda. This pins the intentional FIR-only finding for the "hides" case.
fun customLabelHidesOuterReturn(): Int = run {
    try {
        Unit
    } finally {
        run inner@{ <!ReturnFromFinally!>return@run 1<!> }
    }
    0
}
