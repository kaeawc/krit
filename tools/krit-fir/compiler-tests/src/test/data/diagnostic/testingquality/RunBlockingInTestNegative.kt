// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.runblocking.negative

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

private fun Int.shouldBe(expected: Int) { assert(this == expected) }
private fun assertBoundary(block: () -> Unit) { block() }
private fun verifyBoundary(block: () -> Unit) { block() }
private fun validateBoundary(block: () -> Unit) { block() }
private fun runOnIdle(block: () -> Unit) { block() }
private fun runOnUiThread(block: () -> Unit) { block() }
private fun runOnActivity(block: () -> Unit) { block() }
private fun awaitItem() = 1
private fun await_idle(block: () -> Unit) { block() }
private fun cancelAndIgnoreRemainingEvents() = Unit
private fun Int.isEqualTo(expected: Int) { assert(this == expected) }

class BlockingNegativeTests {
    fun ordinaryFunction() = runBlocking { assert(true) }

    @Test
    fun modernTest() = runTest { assert(true) }

    @Test
    fun boundaries() {
        assertBoundary { runBlocking { assert(true) } }
        verifyBoundary { runBlocking { assert(true) } }
        validateBoundary { runBlocking { assert(true) } }
        runOnIdle { runBlocking { assert(true) } }
        runOnUiThread { runBlocking { assert(true) } }
        runOnActivity { runBlocking { assert(true) } }
    }

    @Test
    fun assertedResult() {
        val result = runBlocking { 1 }
        assert(result == 1)
    }

    @Test
    fun assertedReceiverResult() {
        val result = runBlocking { 1 }
        result.isEqualTo(1)
    }

    // Go exempts receiver assertions; confirm-only drops any unmatched FIR candidate.
    @Test
    fun assertedReceiverCall() { runBlocking { 1 }.shouldBe(1) }

    // Go treats underscore as an ASCII token boundary; confirm-only drops any unmatched FIR candidate.
    @Test
    fun underscoredAssertionBoundary() { await_idle { runBlocking { assert(true) } } }

    @Test
    fun threadIdentity() = runBlocking { Thread.currentThread() }

    @Test
    fun turbineNames() = runBlocking {
        assert(awaitItem() == 1)
        cancelAndIgnoreRemainingEvents()
    }

    @Test
    fun intentionalComment() {
        // Intentionally use runBlocking to test thread confinement.
        runBlocking { assert(true) }
    }

    @Test
    fun sameLineComment() {
        runBlocking { assert(true) } // Deliberate runBlocking thread locking.
    }

    @Test
    fun localHelperIsNotTest() {
        fun helper() = runBlocking { assert(true) }
        helper()
    }
}
