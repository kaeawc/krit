// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 17, 21, 26, 31, 36, 41, 46
package test

data class Box<T>(val value: T)

class KnownThing

class Positives {
    fun nullableDeclaration(input: String?) {
        val result = <!CastNullableToNonNullableType!>input as String<!>
        println(result)
    }

    fun inferredNullable(flag: Boolean): String {
        val value = if (flag) "ok" else null
        return <!CastNullableToNonNullableType!>value as String<!>
    }

    fun genericCast(input: Box<String>?) {
        val result = <!CastNullableToNonNullableType!>input as Box<String><!>
        println(result)
    }

    fun parenthesized(input: String?) {
        val result = <!CastNullableToNonNullableType!>(input) as String<!>
        println(result)
    }

    fun anyOperand(input: Any?) {
        val result = <!CastNullableToNonNullableType!>input as String<!>
        println(result)
    }

    fun safeCallOperand(input: String?) {
        val result = <!CastNullableToNonNullableType!>input?.trim() as String<!>
        println(result)
    }

    fun elvisToNullable(input: String?, other: String?) {
        val result = <!CastNullableToNonNullableType!>(input ?: other) as String<!>
        println(result)
    }

    fun <T> unboundedOperand(input: T) {
        val result = <!CastNullableToNonNullableType!>input as Any<!>
        println(result)
    }
}
