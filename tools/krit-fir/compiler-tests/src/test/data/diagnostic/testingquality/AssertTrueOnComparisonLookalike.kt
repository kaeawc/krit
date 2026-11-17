// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 22
// An assertTrue that is not a test library function. Go reports every receiverless call by name.
package test.asserttrueoncomparison.lookalike

fun assertTrue(condition: Boolean) {}

class Checker {
    fun assertTrue(condition: Boolean) {}
}

object Helpers {
    fun assertTrue(condition: Boolean) {}
}

fun topLevel(actual: Int) {
    assertTrue(actual == 42)
}

fun member(checker: Checker, actual: Int) {
    checker.assertTrue(actual == 42)
    with(checker) { assertTrue(actual == 42) }
}

fun nested(actual: Int) {
    Helpers.assertTrue(actual == 42)
}
