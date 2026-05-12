// Negative: return inside a nested lambda does not make outer statements unreachable.
fun nestedLambda(xs: List<Int>): Int {
    xs.forEach {
        if (it < 0) return@forEach
        println(it)
    }
    return xs.size
}

// Negative: continue inside a loop branch; following code in the SAME branch
// stays after it but only when in the same `statements` block. Here the
// branch body is a single statement, so nothing follows continue.
fun loopContinue(xs: List<Int>) {
    for (x in xs) {
        if (x == 0) continue
        println(x)
    }
}
