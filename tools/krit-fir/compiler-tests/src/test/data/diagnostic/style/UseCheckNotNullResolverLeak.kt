// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27
// Positives Go misses: `x` is a nullable parameter of each function below, so
// check(x != null) should be checkNotNull(x). Go skips them because, in a file
// that declares a class before them, its resolver carries the non-null fact
// that `requireNotNull(x)` establishes in `narrowed` into every later
// function with a parameter named `x`.
package test

class Holder(val value: String?)

fun narrowed(x: String?) {
    requireNotNull(x)
    println(x)
}

fun later(x: String?) {
    <!UseCheckNotNull!>check(x != null)<!>
}

fun laterWithMessage(x: Any?) {
    <!UseCheckNotNull!>check(x != null)<!> { "x must not be null" }
}

// Another name is not affected: Go reports it too.
fun otherName(y: String?) {
    <!UseCheckNotNull!>check(y != null)<!>
}
