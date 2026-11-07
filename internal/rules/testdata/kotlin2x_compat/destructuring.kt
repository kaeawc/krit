// rules: NoNameShadowing, LambdaParameterNaming, DestructuringDeclarationWithTooManyEntries
package compat

data class Point(val x: Int, val y: Int, val z: Int, val w: Int)

fun positionalLoop(m: Map<String, Int>) {
    val k = 1
    for ((k, v) in m) println(k) // expect: NoNameShadowing
}

fun lambdaIsLeftAlone(m: Map<String, Int>) {
    val k = 1
    m.forEach { (k, v) -> println(k) }
}

fun nameBased(p: Point) {
    val x = 1
    run {
        (val x, val y) = p
        println(x)
    }
}

fun tooMany(p: Point) {
    val (a, b, c, d) = p // expect: DestructuringDeclarationWithTooManyEntries
    println(a + b + c + d)
}

fun nameBasedTooMany(p: Point) {
    (val x, val y, val z, val w) = p // expect: DestructuringDeclarationWithTooManyEntries
    println(x + y + z + w)
}

fun lambdaNaming(ps: List<Pair<Int, Int>>) {
    ps.forEach { (X, y) -> println(X) } // expect: LambdaParameterNaming
}
