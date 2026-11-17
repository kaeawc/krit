// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 15, 17, 19, 21
// Positives: toString() on a java.lang.Integer constructor call in the call
// shapes detekt 1.23.8 and Go spell differently. Go reports each one too.
// detekt reports the safe call; it misses the call split across lines, the
// spaced dot, the spaced parentheses (it compares the text after the dot) and
// the call inside a lambda argument (it does not visit call arguments).
package test

fun safeCall(x: Int): String? = <!UnnecessaryTemporaryInstantiation!>Integer(x)<!>?.toString()

fun split(x: Int): String = <!UnnecessaryTemporaryInstantiation!>Integer(x)<!>
    .toString()

fun spacedDot(x: Int): String = <!UnnecessaryTemporaryInstantiation!>Integer(x)<!> . toString()

fun spacedParens(x: Int): String = <!UnnecessaryTemporaryInstantiation!>Integer(x)<!>.toString( )

fun inLambda(xs: List<Int>): List<String> = xs.map { <!UnnecessaryTemporaryInstantiation!>Integer(it)<!>.toString() }

fun chained(x: Int): Int = <!UnnecessaryTemporaryInstantiation!>Integer(x)<!>.toString().length

// Neither reports a toString() whose receiver is not the constructor call.
fun scoped(x: Int): String = Integer(x).run { toString() }

fun withScope(x: Int): String = with(Integer(x)) { toString() }

fun letScope(x: Int): String = Integer(x).let { it.toString() }

fun template(x: Int): String = "${Integer(x)}"
