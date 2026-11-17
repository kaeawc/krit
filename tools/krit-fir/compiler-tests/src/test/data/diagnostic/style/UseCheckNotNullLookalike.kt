// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 30, 54, 59
// Lookalikes: calls named `check` that do not resolve to kotlin.check, so
// checkNotNull is not a replacement for them. The checker requires
// kotlin.check. Go skips a call on a receiver other than `kotlin` and a bare
// call in a class or block that declares a `check` function or property;
// it still reports a `check` reached through a `with` receiver or an
// extension receiver, which it cannot see from the call.
package test

class Validator {
    fun check(value: Boolean) {}
}

object Checks {
    fun check(value: Boolean, message: () -> String = { "" }) {}
}

fun member(validator: Validator, x: Any?) {
    validator.check(x != null)
}

fun objectMember(x: Any?) {
    Checks.check(x != null)
    Checks.check(x != null) { "x must not be null" }
}

fun implicitReceiver(validator: Validator, x: Any?) {
    with(validator) {
        check(x != null)
    }
}

class Subclass {
    private fun check(value: Boolean) {}

    fun validate(x: Any?) {
        check(x != null)
    }
}

fun functionTypedValue(x: Any?) {
    val check: (Boolean) -> Unit = {}
    check(x != null)
}

class Guard

fun Guard.check(value: Boolean) {}

// An extension `check` on the implicit receiver takes priority over the
// default import of kotlin.check.
fun Guard.extensionReceiver(x: Any?) {
    check(x != null)
}

fun extensionInScope(guard: Guard, x: Any?) {
    with(guard) {
        check(x != null)
    }
}
