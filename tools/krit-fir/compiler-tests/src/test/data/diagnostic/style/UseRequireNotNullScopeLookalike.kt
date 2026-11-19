// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 43
// Lookalikes that a scope or import declares: none of these calls is
// kotlin.require. Go skips them as well, because the file shows the
// declaration that the bare name reaches before the default import.
package test

import test.Guards.require as guard

object Guards {
    fun require(value: Boolean, lazyMessage: () -> String = { "" }) {}
}

fun localFunction(x: Any?) {
    fun require(ok: Boolean) {}
    require(x != null)
}

fun localFunctionTypedValue(x: Any?) {
    val require: (Boolean) -> Unit = {}
    require(x != null)
}

class Holder(private val x: Any?) {
    private val require: (Boolean) -> Unit = {}

    fun validate() {
        require(x != null)
    }
}

fun aliasOfAnotherRequire(x: Any?) {
    guard(x != null)
}

// An extension `require` needs a receiver in scope, so this bare call is
// kotlin.require, as Go also reports.
class Target

fun Target.require(ok: Boolean) {}

fun extensionElsewhere(x: Any?) {
    <!UseRequireNotNull!>require(x != null)<!>
}
