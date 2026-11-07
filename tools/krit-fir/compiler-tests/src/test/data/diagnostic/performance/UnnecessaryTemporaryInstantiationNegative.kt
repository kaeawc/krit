// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: neither Go nor FIR reports these.
package test

fun direct(x: Int): String = x.toString()

// detekt only matches the wrapper constructor, not valueOf or parse.
fun conversions(x: Int, s: String) {
    val a = Integer.valueOf(x).toString()
    val b = Integer.parseInt(s).toString()
    val c = java.lang.Integer.valueOf(x).toString()
}

// The constructor is stored first; toString's receiver is a variable.
fun stored(x: Int): String {
    val boxed = Integer(x)
    return boxed.toString()
}

// Other members of the constructed value, and toString with arguments is still toString.
fun otherMembers(x: Int): Int = Integer(x).hashCode()
