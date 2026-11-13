// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Same-named functions that are not assertion-library calls.
package test.assertnullablewithnotnullassertion.lookalike

fun assertEquals(expected: Any?, actual: Any?) {}

fun isEqualTo(value: String?) {}

class Checker {
    fun assertTrue(value: Boolean) {}
    fun isEmpty(value: String?) {}
}

fun localFunction(maybe: String?) {
    assertEquals("x", maybe!!)
    isEqualTo(maybe!!)
}

fun localMember(maybe: String?, checker: Checker) {
    checker.assertTrue(maybe!!.isEmpty())
    checker.isEmpty(maybe!!)
}
