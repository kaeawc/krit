package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UnusedPrivateProperty cases that need request facts: the allowedNames option and the file classification.
class UnusedPrivatePropertyFilesTest {

    private val source = """
        package unusedprivatepropertyfiles

        class Holder(private val tmpCtor: Int, plain: Int) {
            private val tmpValue = 1
            private val ignored = 2
            private val other = 3
        }
    """.trimIndent()

    private fun reported(
        configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
    ): Int {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnusedPrivateProperty"), ruleConfigs = configs),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "UnusedPrivateProperty" && it.file == "F.kt" }
    }

    // Without an option, the default `ignored|expected|serialVersionUID` is a full match, so only `ignored` is allowed.
    @Test fun defaultAllowsOnlyTheListedNames() = assertEquals(4, reported())

    // A configured pattern replaces the default and is a full match too.
    @Test fun configuredPatternIsFullMatch() {
        assertEquals(3, reported(mapOf("UnusedPrivateProperty" to mapOf<String, Any?>("allowedNames" to "tmp.*"))))
        assertEquals(2, reported(mapOf("UnusedPrivateProperty" to mapOf<String, Any?>("allowedNames" to "tmp.*|ignored"))))
        assertEquals(5, reported(mapOf("UnusedPrivateProperty" to mapOf<String, Any?>("allowedNames" to "tmp"))))
    }

    // An invalid pattern falls back to the default instead of failing the file.
    @Test fun invalidPatternFallsBackToDefault() {
        assertEquals(4, reported(mapOf("UnusedPrivateProperty" to mapOf<String, Any?>("allowedNames" to "("))))
    }

    // Go does not skip test files for this rule, so neither does the checker.
    @Test fun testFilesAreNotSkipped() = assertEquals(4, reported(testFiles = setOf("F.kt")))
}
