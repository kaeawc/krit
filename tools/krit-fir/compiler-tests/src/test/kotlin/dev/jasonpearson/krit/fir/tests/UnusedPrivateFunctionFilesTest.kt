package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UnusedPrivateFunction cases that need request facts: the allowedNames option and the file classification.
class UnusedPrivateFunctionFilesTest {

    private val source = """
        package unusedprivatefunctionfiles

        class Holder {
            private fun tmpOne() {}
            private fun tmpTwo() {}
            private fun other() {}
        }
    """.trimIndent()

    private fun reported(
        configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
    ): Int {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnusedPrivateFunction"), ruleConfigs = configs),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "UnusedPrivateFunction" && it.file == "F.kt" }
    }

    private fun allowing(pattern: String) = mapOf("UnusedPrivateFunction" to mapOf<String, Any?>("allowedNames" to pattern))

    // The default option is empty, so nothing is allowed.
    @Test fun defaultAllowsNothing() = assertEquals(3, reported())

    // A configured pattern is a full match, like Go's anchored regex option.
    @Test fun configuredPatternIsFullMatch() {
        assertEquals(1, reported(allowing("tmp.*")))
        assertEquals(0, reported(allowing("tmp.*|other")))
        assertEquals(3, reported(allowing("tmp")))
    }

    // An empty or invalid pattern leaves the option unset in Go, so nothing is allowed.
    @Test fun emptyAndInvalidPatternsAllowNothing() {
        assertEquals(3, reported(allowing("")))
        assertEquals(3, reported(allowing("(")))
    }

    // Go skips test files for this rule, so the checker reports nothing there.
    @Test fun testFilesAreSkipped() = assertEquals(0, reported(testFiles = setOf("F.kt")))
}
