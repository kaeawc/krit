// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 21, 22, 27, 35, 41
// Lookalikes: calls named `check` that do not resolve to kotlin.check, so
// checkNotNull is not a replacement for them. Go matches the callee name alone
// and reports every call below; the checker requires kotlin.check.
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
