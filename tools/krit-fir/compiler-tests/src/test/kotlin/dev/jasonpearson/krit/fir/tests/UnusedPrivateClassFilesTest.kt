package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UnusedPrivateClass cases that need request facts: the file classification.
class UnusedPrivateClassFilesTest {

    private fun reported(sources: Map<String, String>, testFiles: Set<String> = emptySet()): Int {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("UnusedPrivateClass")),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "UnusedPrivateClass" }
    }

    private val unused = mapOf("F.kt" to "package unusedprivateclassfiles\n\nprivate class Hidden\n")

    // Go does not skip test files for this rule, so neither does the checker.
    @Test fun testFilesAreNotSkipped() = assertEquals(1, reported(unused, testFiles = setOf("F.kt")))
}
