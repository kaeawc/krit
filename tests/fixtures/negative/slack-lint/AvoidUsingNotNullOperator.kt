package fixtures.negative.slacklint

// "!!" in a string or comment is not the operator.
fun length(value: String?): Int = value?.length ?: "!!".length
