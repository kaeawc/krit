package test

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull

fun testNullableAssertion(maybeX: String?) {
    assertNotNull(maybeX)
    assertEquals("x", maybeX)
}
