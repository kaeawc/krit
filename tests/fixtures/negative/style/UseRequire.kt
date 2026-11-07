package fixtures.negative.style

fun process(valid: Boolean) {
    require(valid) { "bad" }
    println("processing")
}

// A throw with a cause cannot be expressed with check()/require()
fun processWithCause(cond: Boolean, cause: Exception) {
    if (cond) throw IllegalArgumentException("bad", cause)
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
