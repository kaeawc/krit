package fixtures.positive.style

fun process(valid: Boolean) {
    if (!valid) throw IllegalStateException("bad")
    println("processing")
}
