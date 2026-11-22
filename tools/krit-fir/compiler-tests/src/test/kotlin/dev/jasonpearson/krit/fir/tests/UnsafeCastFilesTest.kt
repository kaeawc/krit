package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class UnsafeCastFilesTest {
    private val context = FirRuleCompileContext(enabledRuleIds = setOf("UnsafeCast"))
    private val source = "fun impossibleCast() = 1 as String"

    @Test fun classifiedTestFileIsExempt() {
        val result = KritFirProbe.compile(mapOf("CustomSource.kt" to source), context, testFiles = setOf("CustomSource.kt"))
        assertTrue(result.clean, result.problems())
        assertTrue(result.diags.isEmpty(), result.diags.toString())
    }

    @Test fun unclassifiedFileReportsWithGoMessage() {
        val result = KritFirProbe.compile(mapOf("Production.kt" to source), context)
        assertTrue(result.clean, result.problems())
        assertEquals(listOf(1), result.diags.map { it.line })
        assertTrue(result.diags.single().message.contains("Cast can never succeed."))
    }

    @Test fun customPreviewOptionsMatchGo() {
        val previewSources = mapOf(
            "Production.kt" to """
                annotation class Theme
                annotation class ScreenPreview

                @Theme fun customPrefix() = 1 as String
                @ScreenPreview fun wildcardName() = 1 as String
            """.trimIndent(),
        )
        val prefixOptions = listOf(
            "Theme",
            "Theme\u0085",
            listOf(" Theme "),
            listOf(mapOf("value" to " Theme ", "reason" to "design-system previews")),
        )
        for (prefixOption in prefixOptions) {
            val configured = context.copy(
                ruleConfigs = mapOf(
                    "UnsafeCast" to mapOf(
                        "customPreviewWildcard" to false,
                        "customPreviewPrefixes" to prefixOption,
                    ),
                ),
            )
            val result = KritFirProbe.compile(previewSources, configured)

            assertTrue(result.clean, result.problems())
            assertEquals(listOf(5), result.diags.map { it.line })
        }
    }
}
