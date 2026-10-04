// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 12, 14, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 32, 33, 34, 40, 41, 46, 49, 58, 59, 60, 61, 66, 69, 73
// Positives: toString() called directly on a JDK wrapper conversion
// (valueOf, parseInt, parseLong, parseFloat, parseDouble). Go reports each of
// these too.
package test

fun convert(x: Int): String {
    return <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x).toString()<!>
}

fun parsed(s: String): String = <!UnnecessaryTemporaryInstantiation!>Integer.parseInt(s).toString()<!>

fun qualified(): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Integer.parseInt("42").toString()<!>

fun everyWrapper(c: Char, b: Byte, s: Short, l: Long, f: Float, d: Double, z: Boolean) {
    val a1 = <!UnnecessaryTemporaryInstantiation!>java.lang.Long.valueOf(l).toString()<!>
    val a2 = <!UnnecessaryTemporaryInstantiation!>java.lang.Long.parseLong("1").toString()<!>
    val a3 = <!UnnecessaryTemporaryInstantiation!>java.lang.Short.valueOf(s).toString()<!>
    val a4 = <!UnnecessaryTemporaryInstantiation!>java.lang.Byte.valueOf(b).toString()<!>
    val a5 = <!UnnecessaryTemporaryInstantiation!>java.lang.Float.valueOf(f).toString()<!>
    val a6 = <!UnnecessaryTemporaryInstantiation!>java.lang.Float.parseFloat("1").toString()<!>
    val a7 = <!UnnecessaryTemporaryInstantiation!>java.lang.Double.valueOf(d).toString()<!>
    val a8 = <!UnnecessaryTemporaryInstantiation!>java.lang.Double.parseDouble("1").toString()<!>
    val a9 = <!UnnecessaryTemporaryInstantiation!>java.lang.Boolean.valueOf(z).toString()<!>
    val a10 = <!UnnecessaryTemporaryInstantiation!>Character.valueOf(c).toString()<!>
    val a11 = <!UnnecessaryTemporaryInstantiation!>java.lang.Character.valueOf(c).toString()<!>
}

// Any argument list counts: a String, a radix.
fun argumentForms(s: String) {
    val b1 = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(s).toString()<!>
    val b2 = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(s, 16).toString()<!>
    val b3 = <!UnnecessaryTemporaryInstantiation!>Integer.parseInt(s, 16).toString()<!>
}

// Any call named toString on the conversion counts, including a radix
// overload and a safe call.
fun toStringForms(x: Int, s: String) {
    val c1 = <!UnnecessaryTemporaryInstantiation!>Integer.parseInt(s).toString(16)<!>
    val c2 = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x)?.toString()<!>
}

// The finding sits on the first line of the chain.
fun multiline(x: Int): String {
    val s = <!UnnecessaryTemporaryInstantiation!>Integer<!>
        .valueOf(x)
        .toString()
    val t = <!UnnecessaryTemporaryInstantiation!>Integer<!>
        .valueOf(x)
        ?.toString()
    return s + t
}

// Enclosing contexts: a string template, an argument, a chained call, a
// lambda, and members of classes, companions, and object expressions.
fun contexts(x: Int): String {
    val t = "v=${<!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x).toString()<!>}"
    println(<!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x).toString()<!>)
    val n = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x).toString()<!>.length
    val f = { y: Int -> <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(y).toString()<!> }
    return t + n + f(x)
}

class Holder(private val x: Int) {
    val text: String get() = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x).toString()<!>

    companion object {
        fun of(x: Int) = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(x).toString()<!>
    }

    val listener = object {
        fun render(y: Int) = <!UnnecessaryTemporaryInstantiation!>Integer.valueOf(y).toString()<!>
    }
}
