package fixtures.positive.style

fun process(valid: Boolean) {
    if (!valid) throw IllegalArgumentException("bad")
    println("processing")
}

fun processWrapped(value: Int) {
    if (!(value > 0)) throw IllegalArgumentException()
    println("processing")
}

fun processNonNegated(cond: Boolean) {
    if (cond) throw IllegalArgumentException()
    println("processing")
}
