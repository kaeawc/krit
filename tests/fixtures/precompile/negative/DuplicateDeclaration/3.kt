// Negative: distinct top-level declarations.
class Foo(val n: Int)

val bar: Int = 1

fun baz(): Int = 0

fun baz(label: String): Int = label.length
