package fixtures.negative.slacklint

fun open(path: String?) {
    requireNotNull(path) { "path is required" }
    check(path.isNotEmpty()) { "path is empty" }
    require(path.length < 10, { "path is too long" })
}

class Validator {
    // A member named like the stdlib precondition.
    private fun check(value: Boolean) = value

    fun validate(value: Boolean) = check(value)
}
