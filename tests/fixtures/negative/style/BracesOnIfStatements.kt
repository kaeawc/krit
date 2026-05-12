package style

fun example(x: Int) {
    if (x > 0) {
        println("positive")
    } else {
        println("non-positive")
    }
}

// Expression-position ifs must not be flagged: wrapping their bodies in
// braces produces visually-broken (though syntactically valid) output.
fun expressionPosition(x: Int): Int {
    val a = if (x > 0) x else -x
    val b: Int = if (x > 0) x else -x
    val c = if (x > 0) {
        x
    } else {
        -x
    }
    return if (x > 0) x else -x + a + b + c
}

fun argument(x: Int) {
    println(if (x > 0) "pos" else "neg")
}

fun singleExprBody(x: Int): Int = if (x > 0) x else -x
