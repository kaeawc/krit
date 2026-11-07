package fixtures.negative.style

fun process(valid: Boolean) {
    check(valid) { "bad" }
    println("processing")
}

// A throw with a cause cannot be expressed with check()/require()
fun processWithCause(cond: Boolean, cause: Exception) {
    if (cond) throw IllegalStateException("bad", cause)
    println("processing")
}

// Wrong exception type should not trigger UseCheckOrError
fun processWrongException(cond: Boolean) {
    if (!cond) throw RuntimeException()
    println("processing")
}
