// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 21, 26
// `x` is a nullable parameter of each function below, so check(x != null)
// should be checkNotNull(x). The non-null fact that `requireNotNull(x)`
// establishes in `narrowed` must not carry into later functions that have their
// own parameter named `x`, even in a file that declares a class first.
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
