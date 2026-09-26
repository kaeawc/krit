// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses these; the receiver is a kotlin.CharArray.
package test

// The receiver's text is not a name Go can type: an indexed element, `x!!` on
// a map value.
fun indexed(arrays: Array<CharArray>): String = <!CharArrayToStringCall!>arrays[0].toString()<!>

fun mapValue(map: Map<String, CharArray>): String = <!CharArrayToStringCall!>map["k"]!!.toString()<!>

// Go does not type `chars` as CharArray from its initializer, an `as?` cast
// with an elvis exit.
fun safeCast(value: Any): String {
    val chars = value as? CharArray ?: return ""
    return <!CharArrayToStringCall!>chars.toString()<!>
}

// Go does not narrow a local var of a property getter in an `object {}`
// expression.
fun objectGetter(input: Any): Any = object {
    val s: String
        get() {
            var v: Any = input
            if (v is CharArray) {
                return <!CharArrayToStringCall!>v.toString()<!>
            }
            return ""
        }
}

// Go does not narrow the branches of an `if` that is the operand of `return`.
fun returnIf(value: Any): String {
    return if (value is CharArray) <!CharArrayToStringCall!>value.toString()<!> else ""
}
