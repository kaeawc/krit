// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 18, 23, 28, 33
package test.testwithoutassertion

import io.mockk.every
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test

class MissingAssertions {
    @Test
    fun <!TestWithoutAssertion!>loads<!>() {
        println(42)
    }

    @Test
    fun
        <!TestWithoutAssertion!>multilineName<!>() {
        println("loaded")
    }

    @Test
    fun coroutineWithoutCheck() = runTest { // FIR abstains: external coroutine runner is not allowlisted.
        println("work")
    }

    @Test
    fun stubbingIsNotVerification() { // FIR abstains: external mocking calls are not allowlisted.
        every { println("stub") } returns Unit
    }

    @Test
    fun <!TestWithoutAssertion!>safeCastIsNotVerification<!>() {
        val value: Any = "value"
        val text = value as? String
    }

    // Go counts an assertion name even when its resolved project body does not assert.
    @Test
    fun projectAssertionLookalike() {
        assertTrue(true)
    }

    // Go's verify text/name heuristic also accepts this project lookalike.
    @Test
    fun projectVerificationLookalike() {
        verify()
    }

    // Go counts isEmpty by name; resolution proves this is a collection query, not an assertion.
    @Test
    fun ordinaryCollectionQuery() {
        listOf(1).isEmpty()
    }

    // Go sees the assertion in a nested declaration that is never called.
    @Test
    fun unusedLocalHelper() {
        fun unused() { assertEquals(1, 2) }
        println("work")
    }

    // Go mistakes a string literal for documentation of a smoke expectation.
    @Test
    fun ordinaryString() {
        println("should not crash")
    }

    // Go merges same-named helper bodies; FIR follows the non-asserting receiver's declaration.
    @Test
    fun helperNameCollision() {
        PlainHelper().consumeReady()
    }

    private fun assertTrue(value: Boolean) { println(value) }
    private fun verify() { println("ordinary work") }

    @Test
    fun verifyStringExemption() { println("verify(") }

    @Test
    fun failureTypeStringExemption() { println("AssertionError") }

    @Test
    fun explicitReceiverExemption() { this.work() }

    private fun work() { println("work") }
}

class PlainHelper {
    fun consumeReady() { println("ordinary work") }
}

class AssertingHelper {
    fun consumeReady() { assertEquals(1, 1) }
}

class OrdinaryOperations {
    // Go accepts the name error without checking that it is Kotlin's throwing function.
    @Test
    fun projectErrorLookalike() { error("ordinary work") }

    // A project infix function named shouldBe is not a Kotest assertion.
    @Test
    fun projectInfixLookalike() { 1 shouldBe 1 }

    private fun error(message: String) { println(message) }
    private infix fun Int.shouldBe(expected: Int) { println(expected) }
}
