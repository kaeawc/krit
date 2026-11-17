// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class Holder {
    var mutableProp: String? = null
}

class Divergences {
    val holder = Holder()

    // A mutable property is not smart cast after the null check, so the compiler types it as nullable. Go does not report it.
    fun mutablePropertyIsNotSmartCast(): String {
        if (holder.mutableProp != null) {
            return <!NullableToStringCall!>holder.mutableProp.toString()<!>
        }
        return ""
    }

    // Go does not type the implicit lambda parameter, which the compiler knows is `String?`.
    fun lambdaParameter(values: List<String?>): List<String> = values.map { <!NullableToStringCall!>it.toString()<!> }

    // Go does not infer that a safe call's result is nullable when it is the receiver of a plain call.
    fun safeCallResult(value: String?): String = <!NullableToStringCall!>value?.trim().toString()<!>

    // Go does not type an unbounded type parameter as nullable.
    fun <T> unboundedReceiver(value: T): String = <!NullableToStringCall!>value.toString()<!>

    // Go does not report a template over a nullable property read through another object.
    fun templateOverMember(): String = "name=<!NullableToStringCall!>${holder.mutableProp}<!>"

    // Go does not type the `message` property of a caught exception, which is `String?`.
    fun caughtMessage(): String {
        try {
            return holder.mutableProp.orEmpty()
        } catch (e: IllegalStateException) {
            return "failed: <!NullableToStringCall!>${e.message}<!>"
        }
    }

    // Go does not type a local initialised by a safe cast.
    fun safeCastLocal(input: Any): String {
        val safe = input as? CharSequence
        return <!NullableToStringCall!>safe.toString()<!>
    }
}
