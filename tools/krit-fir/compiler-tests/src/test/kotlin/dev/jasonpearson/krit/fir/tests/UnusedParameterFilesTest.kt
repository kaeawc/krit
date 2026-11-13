package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UnusedParameter cases that need request facts: the allowedNames option and the file classification.
class UnusedParameterFilesTest {

    private val source = """
        package unusedparameterfiles

        fun f(tmpValue: Int, ignored: Int, other: Int) {
            println("body")
        }
    """.trimIndent()

    private fun reported(
        configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
    ): Int {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnusedParameter"), ruleConfigs = configs),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "UnusedParameter" && it.file == "F.kt" }
    }

    // Without an option, the default `ignored|expected` is a full match, so only `ignored` is allowed.
    @Test fun defaultAllowsOnlyIgnoredAndExpected() = assertEquals(2, reported())

    // A configured pattern replaces the default and is a full match too.
    @Test fun configuredPatternIsFullMatch() {
        assertEquals(2, reported(mapOf("UnusedParameter" to mapOf<String, Any?>("allowedNames" to "tmp.*"))))
        assertEquals(1, reported(mapOf("UnusedParameter" to mapOf<String, Any?>("allowedNames" to "tmp.*|ignored"))))
        assertEquals(3, reported(mapOf("UnusedParameter" to mapOf<String, Any?>("allowedNames" to "tmp"))))
    }

    // An invalid pattern falls back to the default instead of failing the file.
    @Test fun invalidPatternFallsBackToDefault() {
        assertEquals(2, reported(mapOf("UnusedParameter" to mapOf<String, Any?>("allowedNames" to "("))))
    }

    @Test fun testFilesAreSkipped() = assertEquals(0, reported(testFiles = setOf("F.kt")))
}
