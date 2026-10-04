// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 16
// Lookalike: an explicit import of another `check` takes priority over the
// default import of kotlin.check, so the calls below are not kotlin.check.
// Go matches the callee name alone and reports them; the checker does not.
package test

import test.Guards.check

object Guards {
    fun check(value: Boolean, message: () -> String = { "" }) {}
}

fun explicitImport(x: Any?) {
    check(x != null)
    check(x != null) { "x must not be null" }
}
