// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 25, 26, 27, 32, 33, 34, 39, 46, 53
// A one-argument assertTrue over `==` or a null comparison, for each assertion library.
package test.asserttrueoncomparison

import junit.framework.TestCase
import org.junit.Assert
import org.junit.Assert.assertTrue
import org.junit.jupiter.api.Assertions

fun compute(): Int = 42

fun junitEquals() {
    val actual = compute()
    <!AssertTrueOnComparison!>assertTrue<!>(actual == 42)
}

fun junitNotEquals() {
    val actual = compute()
    // `!=` against a non-null value has no dedicated assertion.
    assertTrue(actual != 42)
}

fun nullComparisons(value: String?) {
    <!AssertTrueOnComparison!>assertTrue<!>(value == null)
    <!AssertTrueOnComparison!>assertTrue<!>(value != null)
    <!AssertTrueOnComparison!>assertTrue<!>(null == value)
}

fun qualified() {
    val actual = compute()
    Assert.<!AssertTrueOnComparison!>assertTrue<!>(actual == 42)
    Assertions.<!AssertTrueOnComparison!>assertTrue<!>(actual == 42)
    TestCase.<!AssertTrueOnComparison!>assertTrue<!>(actual == 42)
}

fun multiLine() {
    val actual = compute()
    <!AssertTrueOnComparison!>assertTrue<!>(
        actual == 42,
    )
}

fun namedArgument() {
    val actual = compute()
    <!AssertTrueOnComparison!>assertTrue<!>(condition = actual == 42)
}

class LegacyTest : TestCase() {
    fun testEquals() {
        val actual = compute()
        // Inherited from junit.framework.Assert through TestCase.
        <!AssertTrueOnComparison!>assertTrue<!>(actual == 42)
    }
}
