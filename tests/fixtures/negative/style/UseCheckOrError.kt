package fixtures.negative.style

fun process(valid: Boolean) {
    check(valid) { "bad" }
    println("processing")
}

// Non-negated condition should not trigger
fun processNonNegated(cond: Boolean) {
    if (cond) throw IllegalStateException()
    println("processing")
}

// Multiple statements in block should not trigger
fun processMultiStatement(cond: Boolean) {
    if (!cond) {
        println("log")
        throw IllegalStateException()
    }
}

// Wrong exception type should not trigger UseCheckOrError
fun processWrongException(cond: Boolean) {
    if (!cond) throw RuntimeException()
    println("processing")
}
