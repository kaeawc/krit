// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 27, 32, 42, 50
// Where Go's name-keyed smart casts and the value K2 proves disagree.
package test

// Go misses these: it narrows only the bodies of an `if` and of a `when` with
// a subject, but the value is a CharArray here too.
fun subjectlessWhen(value: Any): String = when {
    value is CharArray -> <!CharArrayToStringCall!>value.toString()<!>
    else -> ""
}

fun rightOperand(value: Any): Boolean = value is CharArray && <!CharArrayToStringCall!>value.toString()<!>.isEmpty()

// Go misses this: it narrows the bare name only, not `this.data`.
class Holder(val data: Any) {
    fun memberThis(): String = if (this.data is CharArray) <!CharArrayToStringCall!>this.data.toString()<!> else ""
}

// Go reports these; the value is not a CharArray, so FIR is correct to drop
// them.
// Go narrows every body of the `if`, the else branch included.
fun elseBranch(value: Any): String = if (value is CharArray) "" else value.toString()

// Go takes the first is-check anywhere in the condition, here under `||`;
// with `flag` true the value can be anything.
fun disjunction(value: Any, flag: Boolean): String = if (flag || value is CharArray) value.toString() else ""

// Go applies the narrowing of `if (x !is CharArray) return` to its whole
// scope, including the statements before the check.
fun beforeEarlyReturn(value: Any): String {
    val before = value.toString()
    if (value !is CharArray) return before
    return ""
}

// Go keys the smart cast by name; `value` is reassigned to a String first.
fun reassigned(input: Any): String {
    var value = input
    if (value is CharArray) {
        value = "text"
        return value.toString()
    }
    return ""
}

// Go keys the smart cast by the name `data`; the check reads this node's
// data, the call reads other.data.
class Node(val data: Any) {
    fun compare(other: Node): String = if (data is CharArray) other.data.toString() else ""
}
