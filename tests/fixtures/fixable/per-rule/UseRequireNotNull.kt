package fixtures.positive.style

fun validate(x: Any?) {
    require(x != null)
    println(x)
}

fun validateReversed(x: Any?) {
    require(null != x)
    println(x)
}

fun validateWithMessage(x: Any?) {
    require(x != null) { "x must not be null" }
    println(x)
}
