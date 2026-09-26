// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: each call below resolves to kotlin.check through an
// import alias, with a single `!=` null comparison of a nullable value. Go
// compares the raw callee text against "check", so `ensure` does not match.
package test

import kotlin.check as ensure

fun importAlias(x: Any?) {
    <!UseCheckNotNull!>ensure(x != null)<!>
}

fun aliasWithMessage(x: Any?) {
    <!UseCheckNotNull!>ensure(x != null)<!> { "x must not be null" }
}
