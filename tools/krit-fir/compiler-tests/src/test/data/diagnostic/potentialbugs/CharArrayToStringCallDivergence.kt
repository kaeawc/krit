// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 41
// Where FIR's resolved receiver type and the Go rule disagree. Go types the
// receiver by looking its text (after the last `.`) up by name, then falls
// back to any same-named parameter or property in the file declared CharArray
// or initialized with charArrayOf, or to a direct charArrayOf(...) receiver.
// FIR reads the receiver's resolved type.
package test

// Go misses these: the receiver is a CharArray, but its text is not a name
// Go can look up (a call, `x!!`, a parenthesized expression, `this`), or the
// receiver is implicit (Go only visits `receiver.toString()`).
fun callResult(text: String): String = <!CharArrayToStringCall!>text.toCharArray().toString()<!>

fun constructorCall(): String = <!CharArrayToStringCall!>CharArray(2).toString()<!>

fun javaArray(): String = <!CharArrayToStringCall!>java.nio.CharBuffer.allocate(2).array().toString()<!>

fun notNull(chars: CharArray?): String = <!CharArrayToStringCall!>chars!!.toString()<!>

fun parenthesized(chars: CharArray): String = <!CharArrayToStringCall!>(chars).toString()<!>

fun CharArray.implicitReceiver(): String = <!CharArrayToStringCall!>toString()<!>

fun CharArray.explicitThis(): String = <!CharArrayToStringCall!>this.toString()<!>

fun withReceiver(chars: CharArray): String = with(chars) { <!CharArrayToStringCall!>toString()<!> }

// Go reports this because `buffer` is declared CharArray in declaresBuffer
// and its file-wide fallback matches the lambda parameter by name; the lambda
// parameter is a String, so the receiver is not a CharArray.
fun declaresBuffer(buffer: CharArray): String = String(buffer)

fun lambdaParameter(items: List<String>): List<String> = items.map { buffer -> buffer.toString() }

// Go reports this because it cannot type `count` and its fallback matches a
// declaration whose initializer contains a charArrayOf call; `count` is the
// array's size, an Int.
fun arraySize(): String {
    val count = charArrayOf('a').size
    return count.toString()
}
