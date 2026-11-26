// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 31, 44
package test.testwithoutassertion.lambdas

import app.cash.turbine.test
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.ComposeContentTestRule
import androidx.compose.ui.test.onNodeWithTag
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test

class ExecutableLambdas(private val rule: ComposeContentTestRule) {
    @Test
    fun composeAssertion() { rule.onNodeWithTag("ready").assertIsDisplayed() }

    @Test
    fun assertionInsideContent() { rule.setContent { assertEquals(1, 1) } }

    @Test
    fun <!TestWithoutAssertion!>contentWithoutAssertion<!>() { rule.setContent {} } // FIR keeps Go's finding: Compose setup asserts nothing.

    @Test
    fun turbineExpectation() = runTest { flowOf(1).test { awaitItem() } }

    @Test
    fun turbineNoEvents() = runTest { flowOf(1).test { expectNoEvents() } }

    @Test
    fun turbineWithoutExpectation() = runTest { flowOf(1).test { println("work") } } // FIR abstains: test and runTest are in the mirrored Go framework-name table.

    @Test
    fun assertionInsideParentheses() = runTest(testBody = { assertEquals(1, 1) })

    @Test
    fun composedJourney() {
        // Reaching here after layout completes is the expectation.
        rule.setContent {}
        rule.waitForIdle()
    }

    @Test
    fun idleAlone() { rule.waitForIdle() } // FIR abstains: Go-compatible waitFor-name gate.

    // Go walks stored lambda declarations, even though this body never invokes the lambda.
    @Test
    fun uninvokedLambda() {
        val unused = { assertEquals(1, 2) }
        println("work")
    }
}
