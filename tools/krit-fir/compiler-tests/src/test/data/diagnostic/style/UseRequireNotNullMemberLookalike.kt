// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25
// Lookalikes: a member `require` that is not kotlin.require, so
// kotlin.requireNotNull is not their replacement. Go skips the call on an
// explicit receiver and the bare call in the class that declares `require`;
// it still reports the call through a `with` receiver (divergence), which it
// cannot see from the call.
package test

class Validator {
    fun require(ok: Boolean) {
        if (!ok) throw IllegalArgumentException()
    }

    fun implicitReceiver(x: Any?) {
        require(x != null)
    }
}

fun explicitReceiver(v: Validator, x: Any?) {
    v.require(x != null)
}

fun withReceiver(x: Any?) = with(Validator()) {
    require(x != null)
}
