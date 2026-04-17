package fixtures.negative.style

fun validate(x: Any?) {
    checkNotNull(x)
    println(x)
}

fun validateWithMessage(x: Any?) {
    checkNotNull(x) { "x must not be null" }
    println(x)
}

fun validateCondition(x: Int) {
    check(x > 0)
}

fun validateEquality(x: Any?, y: Any?) {
    check(x != y)
}

class Validator {
    fun check(value: Boolean) {
        // custom check method, not stdlib
    }
}
