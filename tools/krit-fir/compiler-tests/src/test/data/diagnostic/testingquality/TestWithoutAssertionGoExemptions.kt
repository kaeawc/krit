// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 31, 38, 43
package test.testwithoutassertion.goexemptions

import org.junit.Test
import androidx.compose.ui.test.hasText
import kotlinx.coroutines.test.runTest

class TrailingChecks {
    private fun require(value: Boolean) { println(value) }
    private fun requireNotNull(value: String?): String = "ready"
    @Test fun ready() { require(true) }
    @Test fun assigned() { val value = requireNotNull("ready") }
    @Test fun coroutineReady() = runTest { require(true) }
    @Test fun coroutineAssigned() = runTest { val value = requireNotNull("ready") }
    @Test fun <!TestWithoutAssertion!>notLast<!>() { require(true); println("after") }
}

class ComposeVisibility {
    private fun isBannerDisplayed() = true
    private fun renderBanner() { isBannerDisplayed() }
    @Test fun banner() { isBannerDisplayed() }
    @Test fun delegatedBanner() { renderBanner() }
}

class ConstructionWithExtendedName {
    private fun assertThrowsExactly() { println("ready") }
    private fun parse(value: String) { println(value) }
    @Test fun rejects() { assertThrowsExactly() }
    @Test fun accepts() { parse("ok") }
    @Test fun <!TestWithoutAssertion!>stillNeedsProof<!>() { println("work") }
}

class ConstructionWithComment {
    // assertFails(value) documents the sibling's intended contract.
    private fun parse(value: String) { println(value) }
    @Test fun accepts() { parse("ok") }
    @Test fun <!TestWithoutAssertion!>stillNeedsProof<!>() { println("work") }
}

class OrdinaryConstruction {
    private fun parse(value: String) { println(value) }
    @Test fun <!TestWithoutAssertion!>accepts<!>() { parse("ok") }
}
