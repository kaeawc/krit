// rules: UnconditionalJumpStatementInLoop, LoopWithTooManyJumpStatements
package compat

// A break or continue inside an inline lambda leaves the enclosing loop. It is not a direct statement of the
// loop body, so it never makes the loop run once.
fun lambdaJumps(xs: List<Int>, ys: List<Int>) {
    for (x in xs) {
        ys.forEach { if (it > x) break }
    }
    for (x in xs) {
        run { break }
    }
    for (x in xs) {
        ys.map { continue }
    }
}

fun directJump(xs: List<Int>) {
    for (x in xs) { // expect: UnconditionalJumpStatementInLoop
        println(x)
        break
    }
}

fun manyLambdaJumps(xs: List<Int>) {
    for (x in xs) { // expect: LoopWithTooManyJumpStatements
        xs.forEach {
            if (it == 1) break
            if (it == 2) continue
        }
    }
}

fun labeled(xs: List<Int>, ys: List<Int>) {
    outer@ for (x in xs) { // expect: LoopWithTooManyJumpStatements
        ys.forEach { y ->
            if (y == x) continue@outer
        }
        break
    }
}
