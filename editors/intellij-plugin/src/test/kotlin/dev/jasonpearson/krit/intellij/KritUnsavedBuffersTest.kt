package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertSame

class KritUnsavedBuffersTest {
    private fun finding(file: String, line: Int, rule: String = "MagicNumber") =
        KritFinding(file = file, line = line, column = 1, ruleSet = "style", rule = rule, severity = "warning", message = "m")

    private fun success(vararg findings: KritFinding) =
        KritRunner.AnalyzeOutcome.Success(KritReport(findings.toList()), "")

    // Findings from krit may name files relative to the project root.
    private val canonical: (String) -> String = { if (it.startsWith("/")) it else "/repo/$it" }

    private val onDisk = mapOf(
        "/repo/A.kt" to listOf(finding("/repo/A.kt", 3)),
        "/repo/B.kt" to listOf(finding("/repo/B.kt", 7)),
    )

    @Test
    fun `no unsaved buffers leaves the disk findings untouched`() {
        assertSame(onDisk, KritUnsavedBuffers.overlay(onDisk, emptyMap(), canonical))
    }

    @Test
    fun `buffer findings replace that file's disk findings only`() {
        val merged = KritUnsavedBuffers.overlay(
            onDisk,
            mapOf("/repo/A.kt" to success(finding("A.kt", 10, "MaxLineLength"))),
            canonical,
        )!!
        assertEquals(listOf(finding("A.kt", 10, "MaxLineLength")), merged["/repo/A.kt"])
        assertEquals(onDisk["/repo/B.kt"], merged["/repo/B.kt"])
    }

    @Test
    fun `a buffer whose edits fixed every finding clears that file`() {
        val merged = KritUnsavedBuffers.overlay(onDisk, mapOf("/repo/A.kt" to success()), canonical)!!
        assertEquals(setOf("/repo/B.kt"), merged.keys)
    }

    @Test
    fun `a buffer with findings that is clean on disk gains them`() {
        val merged = KritUnsavedBuffers.overlay(
            onDisk,
            mapOf("/repo/C.kt" to success(finding("/repo/C.kt", 1))),
            canonical,
        )!!
        assertEquals(listOf(finding("/repo/C.kt", 1)), merged["/repo/C.kt"])
    }

    @Test
    fun `a failed buffer keeps its disk findings`() {
        val merged = KritUnsavedBuffers.overlay(
            onDisk,
            mapOf("/repo/A.kt" to KritRunner.AnalyzeOutcome.Failure("boom")),
            canonical,
        )!!
        assertEquals(onDisk, merged)
    }

    @Test
    fun `a cancelled buffer scan publishes nothing`() {
        assertNull(
            KritUnsavedBuffers.overlay(
                onDisk,
                mapOf(
                    "/repo/A.kt" to success(),
                    "/repo/B.kt" to KritRunner.AnalyzeOutcome.Cancelled,
                ),
                canonical,
            ),
        )
    }
}
