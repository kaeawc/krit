// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 18
// A project class named CharArray shadows kotlin.CharArray in this package and
// overrides toString() to return its text; it is not a char array.
package test

class CharArray(private val text: String) {
    override fun toString(): String = text
}

// Go reports these because it matches the receiver's type by the simple name
// CharArray; the receiver is test.CharArray, whose toString() returns the
// text, so FIR is correct to drop them.
fun project(chars: CharArray): String = chars.toString()

fun projectExplicit(): String {
    val chars: CharArray = CharArray("abc")
    return chars.toString()
}
