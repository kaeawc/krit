package fixtures.negative.style

fun validate(x: Any?) {
    requireNotNull(x)
    println(x)
}

fun validateWithMessage(x: Any?) {
    requireNotNull(x) { "x must not be null" }
    println(x)
}

fun validateCondition(x: Int) {
    require(x > 0)
}

fun validateEquality(x: Any?, y: Any?) {
    require(x != y)
}
