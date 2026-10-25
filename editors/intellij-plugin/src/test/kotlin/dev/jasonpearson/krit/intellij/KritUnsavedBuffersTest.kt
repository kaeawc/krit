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

    // Per-file rules the daemon evaluated on the buffers.
    private val perFile = setOf("MagicNumber", "MaxLineLength")

    private val onDisk = mapOf(
        "/repo/A.kt" to listOf(finding("/repo/A.kt", 3), finding("/repo/A.kt", 9, "DeadCode")),
        "/repo/B.kt" to listOf(finding("/repo/B.kt", 7)),
    )

    private fun overlay(outcomes: Map<String, KritRunner.AnalyzeOutcome>, rules: Set<String>? = perFile) =
        KritUnsavedBuffers.overlay(onDisk, outcomes, rules, canonical)

    @Test
    fun `no unsaved buffers leaves the disk findings untouched`() {
        assertSame(onDisk, overlay(emptyMap()))
    }

    @Test
    fun `buffer findings replace only the evaluated rules for that file`() {
        val merged = overlay(mapOf("/repo/A.kt" to success(finding("A.kt", 10, "MaxLineLength"))))!!
        assertEquals(
            listOf(finding("/repo/A.kt", 9, "DeadCode"), finding("A.kt", 10, "MaxLineLength")),
            merged["/repo/A.kt"],
        )
        assertEquals(onDisk["/repo/B.kt"], merged["/repo/B.kt"])
    }

    @Test
    fun `project-scope findings survive a buffer with no per-file findings`() {
        val merged = overlay(mapOf("/repo/A.kt" to success()))!!
        assertEquals(listOf(finding("/repo/A.kt", 9, "DeadCode")), merged["/repo/A.kt"])
    }

    @Test
    fun `buffer findings from rules outside the evaluated set are ignored`() {
        // A type-aware rule the daemon ran on its resolver-free fallback: its
        // disk finding stays and its buffer finding is not added beside it.
        val merged = overlay(mapOf("/repo/A.kt" to success(finding("/repo/A.kt", 9, "DeadCode"))))!!
        assertEquals(listOf(finding("/repo/A.kt", 9, "DeadCode")), merged["/repo/A.kt"])
    }

    @Test
    fun `a file left with no findings is dropped`() {
        val merged = overlay(mapOf("/repo/B.kt" to success()))!!
        assertEquals(setOf("/repo/A.kt"), merged.keys)
    }

    @Test
    fun `a buffer with findings that is clean on disk gains them`() {
        val merged = overlay(mapOf("/repo/C.kt" to success(finding("/repo/C.kt", 1))))!!
        assertEquals(listOf(finding("/repo/C.kt", 1)), merged["/repo/C.kt"])
    }

    @Test
    fun `a failed buffer keeps its disk findings`() {
        assertEquals(onDisk, overlay(mapOf("/repo/A.kt" to KritRunner.AnalyzeOutcome.Failure("boom"))))
    }

    @Test
    fun `a daemon that does not report its rules leaves the disk findings`() {
        assertSame(onDisk, overlay(mapOf("/repo/A.kt" to success()), rules = null))
    }

    @Test
    fun `a cancelled buffer scan publishes nothing`() {
        assertNull(
            overlay(
                mapOf(
                    "/repo/A.kt" to success(),
                    "/repo/B.kt" to KritRunner.AnalyzeOutcome.Cancelled,
                ),
            ),
        )
    }
}
