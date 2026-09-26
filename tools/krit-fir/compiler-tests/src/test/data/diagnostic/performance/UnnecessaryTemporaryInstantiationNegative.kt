// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: neither Go nor FIR reports these.
package test

fun direct(x: Int): String = x.toString()

// The conversion is stored first; toString's receiver is a variable.
fun stored(x: Int): String {
    val boxed = Integer.valueOf(x)
    return boxed.toString()
}

// An implicit receiver is not the conversion call as written.
fun implicit(x: Int): String = with(Integer.valueOf(x)) { toString() }

// A lambda parameter stands between the conversion and toString.
fun viaLet(x: Int): String = Integer.valueOf(x).let { it.toString() }

// Other members of the conversion's result.
fun otherMembers(x: Int): Int = Integer.valueOf(x).hashCode() + Integer.valueOf(x).compareTo(1)

// Other conversions and other static helpers.
fun otherConversions(x: Int, s: String) {
    val a = Integer.toString(x)
    val b = java.lang.Short.parseShort(s).toString()
    val c = java.lang.Boolean.parseBoolean(s).toString()
    val d = Integer.decode(s).toString()
    val f = Integer.valueOf(x)
    val g = Integer.parseInt(s) + 1
}

// A callable reference, not a call of toString.
fun reference(x: Int): () -> String = Integer.valueOf(x)::toString
