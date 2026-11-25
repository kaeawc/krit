// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// The receiver is not proven to be a CharArray, so FIR is correct to drop
// these; Go reports only the first.
package test

class Box(val chars: String)

// Go looks up `chars`, the text after the last `.`, and finds the CharArray
// parameter; box.chars is a String.
fun otherObjectProperty(chars: CharArray, box: Box): String = box.chars.toString() + String(chars)

// Go and FIR agree: `is A, is B` narrows to neither; it may be an IntArray.
fun severalTypes(value: Any): String = when (value) {
    is CharArray, is IntArray -> value.toString()
    else -> ""
}

// Go and FIR agree: the is-check narrows only the then-branch, not the else-if
// branch, where the value is not a CharArray.
fun elseIf(value: Any, flag: Boolean): String = if (value is CharArray) "" else if (flag) value.toString() else ""
