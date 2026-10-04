// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15, 21
// Go reports these; the receiver is not proven to be a CharArray, so FIR is
// correct to drop them.
package test

class Box(val chars: String)

// Go looks up `chars`, the text after the last `.`, and finds the CharArray
// parameter; box.chars is a String.
fun otherObjectProperty(chars: CharArray, box: Box): String = box.chars.toString() + String(chars)

// Go takes the first is-check of the branch; the value may be an IntArray.
fun severalTypes(value: Any): String = when (value) {
    is CharArray, is IntArray -> value.toString()
    else -> ""
}

// Go narrows every body of the `if`, the else-if branch included, where the
// value is not a CharArray.
fun elseIf(value: Any, flag: Boolean): String = if (value is CharArray) "" else if (flag) value.toString() else ""
