// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 10, 12, 16, 21, 24, 28, 31, 33, 38, 40, 43, 50, 54, 58, 62, 68, 70, 74, 77, 79
// Positives: a zero-argument toString() on a kotlin.CharArray receiver, which
// renders the array's identity. Every shape here is reported by Go as well.
package test

fun parameter(chars: CharArray): String = <!CharArrayToStringCall!>chars.toString()<!>

// A nullable receiver resolves to the stdlib extension Any?.toString().
fun nullableParameter(chars: CharArray?): String = <!CharArrayToStringCall!>chars.toString()<!>

fun safeCall(chars: CharArray?): String? = <!CharArrayToStringCall!>chars?.toString()<!>

fun localInferred(): String {
    val chars = charArrayOf('a', 'b')
    return <!CharArrayToStringCall!>chars.toString()<!>
}

fun localExplicit(): String {
    val chars: CharArray = charArrayOf('a', 'b')
    return <!CharArrayToStringCall!>chars.toString()<!>
}

fun directFactory(): String = <!CharArrayToStringCall!>charArrayOf('a', 'b').toString()<!>

// The finding is on the line where the call expression, receiver included,
// starts.
fun multiLine(chars: CharArray): String = <!CharArrayToStringCall!>chars<!>
    .toString()

fun inLambda(chars: CharArray): String = run { <!CharArrayToStringCall!>chars.toString()<!> }

fun inTemplate(chars: CharArray): String = "value: ${<!CharArrayToStringCall!>chars.toString()<!>}"

class Holder(val chars: CharArray) {
    val buffer: CharArray = CharArray(4)

    fun render(): String = <!CharArrayToStringCall!>chars.toString()<!>

    fun renderBuffer(): String = <!CharArrayToStringCall!>buffer.toString()<!>

    companion object {
        fun of(chars: CharArray): String = <!CharArrayToStringCall!>chars.toString()<!>
    }
}

object Registry {
    val password = charArrayOf('p', 'w')

    fun dump(): String = <!CharArrayToStringCall!>password.toString()<!>
}

interface Renderer {
    fun render(chars: CharArray): String = <!CharArrayToStringCall!>chars.toString()<!>
}

fun anonymous(): Any = object {
    fun render(chars: CharArray): String = <!CharArrayToStringCall!>chars.toString()<!>
}

fun local(): String {
    fun inner(chars: CharArray): String = <!CharArrayToStringCall!>chars.toString()<!>
    return inner(CharArray(1))
}

typealias Chars = CharArray

fun alias(chars: Chars): String = <!CharArrayToStringCall!>chars.toString()<!>

fun smartCast(value: Any): String = if (value is CharArray) <!CharArrayToStringCall!>value.toString()<!> else ""

fun constructorLocal(): String {
    val raw = CharArray(4)
    return <!CharArrayToStringCall!>raw.toString()<!>
}

fun member(holder: Holder): String = <!CharArrayToStringCall!>holder.chars.toString()<!>

fun safeMember(holder: Holder?): String? = <!CharArrayToStringCall!>holder?.chars?.toString()<!>
