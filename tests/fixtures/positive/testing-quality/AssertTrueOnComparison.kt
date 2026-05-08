package test

import org.junit.Assert.assertTrue

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertTrue(actual == expected)
}
