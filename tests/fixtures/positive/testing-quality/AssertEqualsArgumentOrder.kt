package test

import org.junit.Assert.assertEquals

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertEquals(actual, expected)
}
