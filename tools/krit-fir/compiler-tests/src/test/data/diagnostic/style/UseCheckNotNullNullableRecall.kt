// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: each call below is kotlin.check with a single `!=` null
// comparison of a value whose type is nullable at the call, so it should be
// checkNotNull.
package test

// Go compares the raw operand text `(null)` against "null".
fun parenthesizedNull(x: Any?) {
    <!UseCheckNotNull!>check(x != (null))<!>
}

// An unstable smart cast by simple name: a mutable property can change after
// the early return, so K2 keeps its nullable type. Go's resolver narrows the
// name after the early return and skips it.
var topLevel: String? = null

fun topLevelVar() {
    if (topLevel == null) return
    <!UseCheckNotNull!>check(topLevel != null)<!>
}

class Session {
    var token: String? = null

    fun memberVar() {
        if (token == null) return
        <!UseCheckNotNull!>check(token != null)<!>
    }
}
