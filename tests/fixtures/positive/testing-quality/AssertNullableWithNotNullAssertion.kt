package test

import org.junit.Assert.assertEquals

fun testNullableAssertion(maybeX: String?) {
    assertEquals("x", maybeX!!)
}
