package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UnusedVariable cases that need request facts: the allowedNames option and the file classification.
class UnusedVariableFilesTest {

    private val source = """
        package unusedvariablefiles

        fun f() {
            val tmpValue = 1
            val ignored = 2
            val other = 3
        }
    """.trimIndent()

    private fun reported(
        configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
    ): Int {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnusedVariable"), ruleConfigs = configs),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "UnusedVariable" }
    }

    // Without an option, the default `ignored|_` is a full match, so only `ignored` is allowed.
    @Test fun defaultAllowsOnlyIgnored() = assertEquals(2, reported())

    // A configured pattern replaces the default and is a full match too.
    @Test fun configuredPatternIsFullMatch() {
        val configs = mapOf("UnusedVariable" to mapOf<String, Any?>("allowedNames" to "tmp.*"))
        assertEquals(2, reported(configs))
        assertEquals(1, reported(mapOf("UnusedVariable" to mapOf<String, Any?>("allowedNames" to "tmp.*|ignored"))))
        assertEquals(3, reported(mapOf("UnusedVariable" to mapOf<String, Any?>("allowedNames" to "tmp"))))
    }

    // An invalid pattern falls back to the default instead of failing the file.
    @Test fun invalidPatternFallsBackToDefault() {
        assertEquals(2, reported(mapOf("UnusedVariable" to mapOf<String, Any?>("allowedNames" to "("))))
    }

    @Test fun testFilesAreSkipped() = assertEquals(0, reported(testFiles = setOf("F.kt")))
}
