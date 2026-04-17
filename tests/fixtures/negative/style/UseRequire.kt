package fixtures.negative.style

fun process(valid: Boolean) {
    require(valid) { "bad" }
    println("processing")
}
