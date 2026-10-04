// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: the characters are rendered, the receiver is not a CharArray, or
// the call is not a toString() call.
package test

fun constructor(chars: CharArray): String = String(chars)

fun concat(chars: CharArray): String = chars.concatToString()

fun content(chars: CharArray): String = chars.contentToString()

fun joined(chars: CharArray): String = chars.joinToString("")

fun string(text: String): String = text.toString()

fun otherArray(ints: IntArray): String = ints.toString()

fun element(chars: CharArray): String = chars[0].toString()

fun size(chars: CharArray): String = chars.size.toString()

fun boxedArray(chars: Array<Char>): String = chars.toString()

fun reference(chars: CharArray): () -> String = chars::toString

fun template(chars: CharArray): String = "value: ${String(chars)}"

fun listOfArrays(): String {
    val arrays = listOf(charArrayOf('a'))
    return arrays.toString()
}

fun nullableString(text: String?): String = text.toString()

class Wrapper(private val chars: CharArray) {
    override fun toString(): String = String(chars)

    fun render(): String = toString()
}
