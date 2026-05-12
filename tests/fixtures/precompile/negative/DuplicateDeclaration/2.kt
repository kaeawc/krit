// Negative: same-named functions in nested scopes are not top-level
// duplicates; the rule only walks direct children of source_file.
class Outer {
    fun greet(name: String): String = "hi $name"
}

object Helpers {
    fun greet(name: String): String = "hello $name"
}

fun greet(name: String): String = "top-level"
