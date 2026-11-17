// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalike: an explicit import of another `check` takes priority over the
// default import of kotlin.check, so the calls below are not kotlin.check.
// Neither Go nor the checker reports them: Go skips a name the file imports
// from another declaration.
package test

import test.Guards.check

object Guards {
    fun check(value: Boolean, message: () -> String = { "" }) {}
}

fun explicitImport(x: Any?) {
    check(x != null)
    check(x != null) { "x must not be null" }
}
