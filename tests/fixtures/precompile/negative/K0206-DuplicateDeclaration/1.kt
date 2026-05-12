// Negative: distinct overloads — different parameter types.
fun greet(name: String): String = "hi $name"

fun greet(name: Int): String = "hi #$name"

fun greet(first: String, last: String): String = "hi $first $last"
