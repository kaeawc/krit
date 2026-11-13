package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The rule's options come from the check request, and a test file keeps `offer`, `subscribe` and `test`
// results that tests drop on purpose.
class IgnoredReturnValueOptionsTest {

    private val source = """
        package options

        import io.reactivex.rxjava3.processors.PublishProcessor

        fun lazy(items: Sequence<Int>) {
            items.map { it + 1 }
        }

        fun offered(processor: PublishProcessor<String>) {
            processor.offer("a")
        }

        fun skipped(items: Sequence<Int>) {
            items.filter { it > 1 }
        }
    """.trimIndent()

    private fun lines(
        options: Map<String, Any?>? = null,
        testFiles: Set<String> = emptySet(),
    ): List<Int> {
        val result = KritFirProbe.compile(
            mapOf("Options.kt" to source),
            FirRuleCompileContext(
                enabledRuleIds = setOf("IgnoredReturnValue"),
                ruleConfigs = options?.let { mapOf("IgnoredReturnValue" to it) }.orEmpty(),
            ),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "IgnoredReturnValue" }.map { it.line }.sorted()
    }

    @Test fun defaultsReportEverything() {
        assertEquals(listOf(6, 10, 14), lines())
    }

    @Test fun testFileKeepsOfferResult() {
        assertEquals(listOf(6, 14), lines(testFiles = setOf("Options.kt")))
    }

    @Test fun ignoreFunctionCallSkipsByName() {
        assertEquals(listOf(10, 14), lines(mapOf("ignoreFunctionCall" to listOf("map"))))
    }

    @Test fun builtinAnnotationsCountWhateverTheOption() {
        val options = mapOf("returnValueAnnotations" to listOf("NoSuchAnnotation"), "ignoreReturnValueAnnotations" to emptyList<String>())
        assertEquals(listOf(6, 10, 14), lines(options))
    }

    @Test fun ignoreAnnotationOptionSkipsTheAnnotatedCallee() {
        val options = mapOf("ignoreReturnValueAnnotations" to listOf("*.CheckReturnValue"))
        assertEquals(listOf(6, 14), lines(options))
    }
}
