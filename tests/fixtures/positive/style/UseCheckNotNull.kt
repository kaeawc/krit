package fixtures.positive.style

fun validate(x: Any?) {
    check(x != null)
    println(x)
}

fun validateReversed(x: Any?) {
    check(null != x)
    println(x)
}

fun validateWithMessage(x: Any?) {
    check(x != null) { "x must not be null" }
    println(x)
}
