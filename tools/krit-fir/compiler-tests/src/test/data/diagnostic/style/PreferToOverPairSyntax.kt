// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 9, 10, 11, 12, 13, 19x3, 24, 25, 26, 29, 31, 37
// Positive: each call to the kotlin.Pair constructor, in any spelling, should
// be an infix `to`.
package test

fun plain(a: Int, b: String) {
    val p1 = <!PreferToOverPairSyntax!>Pair<!>(a, b)
    val p2 = <!PreferToOverPairSyntax!>Pair<!>(1, 2)
    val p3 = kotlin.<!PreferToOverPairSyntax!>Pair<!>(a, b)
    val p4 = <!PreferToOverPairSyntax!>Pair<!><Int, String>(a, b)
    val p5 = <!PreferToOverPairSyntax!>Pair<!>(first = a, second = b)
    val p6 = <!PreferToOverPairSyntax!>Pair<!>(a, b).copy(first = 3)
    println(listOf(p1, p2, p3, p4, p5, p6))
}

// Nested pairs report once per constructor call.
fun nested() {
    val n = <!PreferToOverPairSyntax!>Pair<!>(<!PreferToOverPairSyntax!>Pair<!>(1, 2), <!PreferToOverPairSyntax!>Pair<!>(3, 4))
    println(n)
}

// Calls used as arguments, in lambdas, defaults and members.
class Holder(val p: Pair<Int, Int> = <!PreferToOverPairSyntax!>Pair<!>(0, 0)) {
    fun f() = <!PreferToOverPairSyntax!>Pair<!>(1, "x")
    val q get() = <!PreferToOverPairSyntax!>Pair<!>(2, 3)
}

fun inLambda(xs: List<Int>) = xs.map { <!PreferToOverPairSyntax!>Pair<!>(it, it) }

fun multiline() = <!PreferToOverPairSyntax!>Pair<!>(
    1,
    2,
)

// A trailing lambda is the second argument.
fun trailingLambda(a: Int) = <!PreferToOverPairSyntax!>Pair<!>(a) { a }
