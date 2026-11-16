// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class Divergences {
    var mutableProp: String? = null

    // Go (source inference, no compiler facts) does not resolve these targets (CharSequence, a bounded T), so it
    // reports neither cast; the compiler does.
    fun upcastToSuper(input: String?) {
        val result = <!CastNullableToNonNullableType!>input as CharSequence<!>
        println(result)
    }

    @Suppress("UNCHECKED_CAST")
    fun <T : Any> boundedTarget(input: Any?): T {
        return <!CastNullableToNonNullableType!>input as T<!>
    }

    // A mutable property is not smart cast after the null check, so the operand is still nullable. Go does not
    // report it.
    fun mutablePropertyIsNotSmartCast() {
        if (mutableProp != null) {
            val result = <!CastNullableToNonNullableType!>mutableProp as String<!>
            println(result)
        }
    }
}
