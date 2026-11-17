// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Calls and comparisons the rule leaves alone.
package test.asserttrueoncomparison.negative

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue

fun compute(): Int = 42

fun other(actual: Int, flag: Boolean) {
    assertTrue(actual > 0)
    assertTrue(actual <= 0)
    assertTrue(flag)
    assertTrue(!flag)
    assertTrue(actual > 0 && flag)
    assertFalse(actual == 42)
    assertEquals(42, actual)
}

fun withMessage(actual: Int) {
    // Only the one-argument form is reported.
    assertTrue("message", actual == 42)
}

fun booleanLiteral(flag: Boolean) {
    // Comparing against a boolean literal is a null-safe idiom, not value equality.
    assertTrue(flag == true)
    assertTrue(flag == false)
    assertTrue(true == flag)
}

fun identity(first: Any, second: Any) {
    assertTrue(first === second)
    assertTrue(first !== second)
}

fun differentCall(actual: Int) {
    println(actual == 42)
    check(actual == 42)
}
