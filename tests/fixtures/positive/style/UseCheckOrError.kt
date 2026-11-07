package fixtures.positive.style

fun process(valid: Boolean) {
    if (!valid) throw IllegalStateException("bad")
    println("processing")
}

fun processWrapped(value: Int) {
    if (!(value > 0)) throw IllegalStateException()
    println("processing")
}

fun processNonNegated(cond: Boolean) {
    if (cond) throw IllegalStateException()
    println("processing")
}
