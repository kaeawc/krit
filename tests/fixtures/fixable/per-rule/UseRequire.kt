package fixtures.positive.style

fun process(valid: Boolean) {
    if (!valid) throw IllegalArgumentException("bad")
    println("processing")
}
