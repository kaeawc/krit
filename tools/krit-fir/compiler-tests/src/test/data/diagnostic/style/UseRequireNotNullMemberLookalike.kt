// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 20, 24
// Lookalikes: a member `require` that is not kotlin.require. Divergence on
// every call below: Go reports any call named `require`, including a member
// call through an explicit or implicit receiver; none of them is
// kotlin.require, so kotlin.requireNotNull is not their replacement.
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
