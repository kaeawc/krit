package fixtures.negative.style

fun check(x: String?) {
    if (x.isNullOrEmpty()) {
        println("nothing")
    }
}
