// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 15
// A project extension CharArray?.toString() is more specific than the stdlib
// Any?.toString(), so a nullable receiver calls it and gets the characters.
package test

fun CharArray?.toString(): String = if (this == null) "" else String(this)

// Go reports this because it checks only the call's name and the receiver's
// type; the call resolves to the project extension above, which returns the
// characters, so FIR is correct to drop it.
fun projectExtension(chars: CharArray?): String = chars.toString()

// A non-null receiver still calls the member inherited from Any.
fun member(chars: CharArray): String = <!CharArrayToStringCall!>chars.toString()<!>
