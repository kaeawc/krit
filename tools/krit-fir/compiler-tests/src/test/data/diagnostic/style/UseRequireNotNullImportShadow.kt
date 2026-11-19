// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalike: an explicit import of another `require` takes priority over the
// default import of kotlin.require, so the calls below are not kotlin.require.
// Neither Go nor the checker reports them: Go skips a name the file imports
// from another declaration.
package test

import test.Guards.require

object Guards {
    fun require(value: Boolean, lazyMessage: () -> String = { "" }) {}
}

fun explicitImport(x: Any?) {
    require(x != null)
    require(x != null) { "x must not be null" }
}
