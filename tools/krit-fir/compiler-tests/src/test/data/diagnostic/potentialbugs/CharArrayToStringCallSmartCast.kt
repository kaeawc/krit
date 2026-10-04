// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 10, 14, 22, 31, 37, 42, 48, 55, 60, 66
// A value smart-cast to CharArray by an is-check. K2 drops the smart cast
// because Any.toString() applies to the declared type, so the checker
// recovers the check from the enclosing code.
package test

fun parameter(value: Any): String = if (value is CharArray) <!CharArrayToStringCall!>value.toString()<!> else ""

fun nullable(value: Any?): String = if (value is CharArray) <!CharArrayToStringCall!>value.toString()<!> else ""

fun conjunction(value: Any, flag: Boolean): String {
    if (flag && value is CharArray) {
        return <!CharArrayToStringCall!>value.toString()<!>
    }
    return ""
}

fun localVal(input: Any): String {
    val value = input
    if (value is CharArray) {
        return <!CharArrayToStringCall!>value.toString()<!>
    }
    return ""
}

fun localVar(input: Any): String {
    var value = input
    println(value)
    if (value is CharArray) {
        return <!CharArrayToStringCall!>value.toString()<!>
    }
    return ""
}

fun whenSubject(value: Any): String = when (value) {
    is CharArray -> <!CharArrayToStringCall!>value.toString()<!>
    else -> ""
}

fun whenSubjectVariable(input: Any): String = when (val value = input) {
    is CharArray -> <!CharArrayToStringCall!>value.toString()<!>
    else -> ""
}

fun earlyReturn(value: Any): String {
    if (value !is CharArray) return ""
    return <!CharArrayToStringCall!>value.toString()<!>
}

fun earlyThrow(value: Any): String {
    if (value !is CharArray) {
        throw IllegalArgumentException("not chars")
    }
    return <!CharArrayToStringCall!>value.toString()<!>
}

fun inLambda(value: Any): String {
    if (value is CharArray) {
        return run { <!CharArrayToStringCall!>value.toString()<!> }
    }
    return ""
}

class Holder(val data: Any) {
    fun member(): String = if (data is CharArray) <!CharArrayToStringCall!>data.toString()<!> else ""
}

// Negatives: no is-check proves the value is a CharArray here.
fun negated(value: Any): String = if (value !is CharArray) value.toString() else ""

fun otherArray(value: Any): String = if (value is IntArray) value.toString() else ""

fun otherValue(value: Any, other: Any): String = if (value is CharArray) other.toString() else ""
