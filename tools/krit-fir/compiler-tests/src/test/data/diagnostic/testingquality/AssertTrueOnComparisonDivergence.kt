// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Shapes the checker reports and Go misses.
package test.asserttrueoncomparison.divergence

import org.junit.Assert.assertTrue as check
import org.junit.Assert

fun compute(): Int = 42

// Go matches the written name, so an import alias hides the call.
fun importAlias() {
    val actual = compute()
    <!AssertTrueOnComparison!>check<!>(actual == 42)
}

// Go requires the argument to be the comparison itself; the parentheses hide it.
fun parenthesized() {
    val actual = compute()
    Assert.<!AssertTrueOnComparison!>assertTrue<!>((actual == 42))
}
