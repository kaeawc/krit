package fixtures.negative.style

fun process(valid: Boolean) {
    require(valid) { "bad" }
    println("processing")
}

// Non-negated condition should not trigger
fun processNonNegated(cond: Boolean) {
    if (cond) throw IllegalArgumentException()
    println("processing")
}

// Multiple statements in block should not trigger
fun processMultiStatement(cond: Boolean) {
    if (!cond) {
        println("log")
        throw IllegalArgumentException()
    }
}

// Wrong exception type should not trigger UseRequire
fun processWrongException(cond: Boolean) {
    if (!cond) throw RuntimeException()
    println("processing")
}
