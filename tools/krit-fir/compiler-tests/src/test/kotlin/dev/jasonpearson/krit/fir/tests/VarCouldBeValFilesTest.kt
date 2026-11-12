package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// VarCouldBeVal cases that need request facts: the ignoreLateinitVar option and the file classification.
class VarCouldBeValFilesTest {

    private val source = """
        package varcouldbevalfiles

        class Holder {
            private lateinit var provider: String
            private var plain = 1

            fun read() = provider + plain
        }
    """.trimIndent()

    private fun reported(
        configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
    ): Int {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("VarCouldBeVal"), ruleConfigs = configs),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "VarCouldBeVal" }
    }

    // Go's default for the option is false, so a lateinit var is examined like any other.
    @Test fun lateinitIsExaminedByDefault() = assertEquals(2, reported())

    @Test fun ignoreLateinitVarSkipsIt() {
        val configs = mapOf("VarCouldBeVal" to mapOf<String, Any?>("ignoreLateinitVar" to true))
        assertEquals(1, reported(configs))
    }

    @Test fun testFilesAreSkipped() = assertEquals(0, reported(testFiles = setOf("F.kt")))
}
