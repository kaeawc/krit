package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// ReturnFromFinally ignoreLabeled: a return written with a label is skipped, even when it leaves the function.
class ReturnFromFinallyIgnoreLabeledTest {

    private val source = """
        package ignorelabeled

        fun plain(): Int {
            try { println() } finally { return 1 }
        }

        fun labeled(): Int {
            try { println() } finally { return@labeled 2 }
        }
    """.trimIndent()

    private fun reported(options: Map<String, Any?>): List<Int> {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(
                enabledRuleIds = setOf("ReturnFromFinally"),
                ruleConfigs = mapOf("ReturnFromFinally" to options),
            ),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "ReturnFromFinally" && it.file == "F.kt" }.map { it.line }.sorted()
    }

    @Test fun defaultReportsLabeledReturnsToo() = assertEquals(listOf(4, 8), reported(emptyMap()))

    @Test fun ignoreLabeledSkipsWrittenLabels() = assertEquals(listOf(4), reported(mapOf("ignoreLabeled" to true)))
}
