package fixtures.negative.style

fun process(valid: Boolean) {
    check(valid) { "bad" }
    println("processing")
}
