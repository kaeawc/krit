// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 26, 32, 36, 43
package test.testwithoutassertion.divergence

import org.junit.Assert.assertEquals as compareValues
import org.junit.Test
import org.junit.Test as Spec
import io.mockk.verify as observeCalls
import kotlinx.coroutines.test.runTest as execute

class ResolvedDeclarations {
    // Go does not recognize the written assertion alias; FIR resolves JUnit's assertEquals.
    @Test
    fun aliasedAssertion() { compareValues(1, 1) } // FIR abstains: resolved JUnit assertion alias.

    // FIR preserves Go's annotation-spelling gate even when the alias resolves to Test.
    @Spec
    fun aliasedTestAnnotation() { println("work") }

    // FIR preserves Go's expected-argument exemption, including explicit Test.None.
    @Test(expected = Test.None::class)
    fun noExpectedException() { println("work") }

    // Go's ensure-name gate exempts this call before inspecting the helper's body.
    @Test
    fun resolvedConditionHelper() { ensureReady(true) } // FIR abstains: Go-compatible ensure-name gate.

    private fun ensureReady(ready: Boolean) { if (!ready) error("not ready") }

    // The written alias hides MockK verification from Go's name checks.
    @Test
    fun aliasedVerification() { observeCalls { println("work") } } // FIR abstains: resolved MockK verification alias.

    // Go cannot unwrap an aliased runner to recognize the trailing throwing cast.
    @Test
    fun aliasedRunnerCast() = execute { // FIR abstains: unknown runner and throwing cast.
        val value: Any = "value"
        value as String
    }

    // A resolved require still checks a condition when it is not the final statement; Go only accepts it last.
    @Test
    fun earlyConditionCheck() { // FIR abstains: resolved non-trailing require checks the condition.
        require(true)
        println("work")
    }
}
