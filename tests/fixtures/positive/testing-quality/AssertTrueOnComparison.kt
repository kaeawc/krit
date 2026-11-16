package test

import org.junit.Assert.assertTrue

fun compute(): Int = 42

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertTrue(actual == expected)
}
