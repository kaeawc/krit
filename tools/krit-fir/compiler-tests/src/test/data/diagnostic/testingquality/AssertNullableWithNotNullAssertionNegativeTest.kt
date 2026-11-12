// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives that match Go.
package test.assertnullablewithnotnullassertion.negative

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull

fun lookup(value: String): String = value

fun nonNullOperand(value: String) {
    @Suppress("UNNECESSARY_NOT_NULL_ASSERTION")
    assertEquals("x", value!!)
}

fun notNullAssertionItself(maybe: String?) {
    // assertNotNull and assertNull take the nullable value, so `!!` there is not a nullable assertion.
    assertNotNull(maybe!!)
    assertNull(maybe!!)
}

fun nestedCallArgument(maybe: String?) {
    // The `!!` is an argument of lookup, not the assertion's own subject.
    assertEquals("x", lookup(maybe!!))
}

fun insideLambda(maybe: String?) {
    assertEquals("x", maybe.let { it!! })
    assertEquals("x", run { maybe!! })
}

fun checkedFirst(maybe: String?) {
    // Without a Kotlin contract, `!!` after the check is the required idiom.
    assertNotNull(maybe)
    assertEquals("x", maybe!!)
}

fun checkedWithMessage(maybe: String?) {
    assertNotNull("must exist", maybe)
    assertEquals("x", maybe!!)
}

fun checkedInEnclosingBlock(maybe: String?, flag: Boolean) {
    assertNotNull(maybe)
    if (flag) {
        assertEquals("x", maybe!!)
    }
}

fun checkedByRequire(maybe: String?) {
    requireNotNull(maybe)
    assertEquals("x", maybe!!)
}

fun noAssertionCall(maybe: String?) {
    val local = maybe!!
    assertEquals("x", local)
}

fun platformType() {
    assertEquals("x", System.getProperty("user.name")!!)
}
