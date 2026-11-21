package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class TestWithoutAssertionIncrementalTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private fun session(planning: Boolean = true): AnalysisSession =
        AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = planning, incrementalPlanning = planning)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); sessions += it }

    private fun AnalysisSession.checkAll(): BatchResult = check(1, kotlinFilesIn(src).map { FileRef(it) }, emptySet())

    private fun module() {
        src.writeSource("Test.kt", "package org.junit\n\nannotation class Test\n")
        src.writeSource("Helpers.kt", "package p\n\nfun verified() { require(true) }\nfun unverified() { println(1) }\n")
        src.writeSource("Checks.kt", """
            package p
            import org.junit.Test
            class Checks {
                @Test fun externalAssertion() { verified() }
                @Test fun externalOperation() { unverified() }
                @Test fun localAssertion() { condition() }
                @Test fun localOperation() { work() }
                private fun condition() { require(true) }
                private fun work() { println(1) }
            }
        """.trimIndent())
    }

    private fun assertSameAsFull(got: BatchResult) {
        val want = session(planning = false).checkAll()
        assertTrue(got.errorFiles.isEmpty(), got.errorFiles.toString())
        assertTrue(got.ruleErrors.isEmpty(), got.ruleErrors.toString())
        for (path in kotlinFilesIn(src)) assertEquals(want.verdictOf(path).key(), got.verdictOf(path).key(), path)
        val findings = got.findings.filter { it.rule == "TestWithoutAssertion" }
        assertEquals(listOf(7), findings.map { it.line })
        assertEquals("Test function has no assertion; add a verification.", findings.single().message)
    }

    @Test fun allRulesStillBuildASnapshotAndCheckAgainstBinaryHelpers() {
        module()
        val planned = session()
        val cold = planned.checkAll()
        assertEquals("cold", cold.plan?.reason)
        assertNull(planned.sliceFallback())
        assertSameAsFull(cold)

        val checks = File(src, "Checks.kt")
        checks.writeText(checks.readText().replace("work() { println(1) }", "work() { println(2) }"))
        val warm = planned.checkAll()
        assertEquals(PlanPath.INCREMENTAL, planned.lastPlan?.path)
        assertEquals(1, planned.lastPlan?.dirtyFiles)
        assertSameAsFull(warm)
    }

    @Test fun aCrossFileHelperBodyEditDoesNotChangeCachedVerdicts() {
        module()
        val planned = session()
        assertSameAsFull(planned.checkAll())

        src.writeSource("Helpers.kt", "package p\n\nfun verified() { println(1) }\nfun unverified() { require(true) }\n")
        val warm = planned.checkAll()
        assertEquals(PlanPath.INCREMENTAL, planned.lastPlan?.path)
        assertEquals(1, planned.lastPlan?.dirtyFiles)
        assertSameAsFull(warm)
    }

    @Test fun aDirectSliceAndAFullCompileGiveTheSameHelperVerdicts() {
        module()
        val recording = recordingSession(src, tmp.resolve("direct-snapshots").toFile()).also { sessions += it }
        val full = recording.checkAll()
        assertSameAsFull(full)
        assertNull(recording.sliceFallback())
        val path = canon(File(src, "Checks.kt").path)
        val sliced = recording.sliceCheck(2, listOf(FileRef(path)), emptySet())
        assertTrue(sliced is SliceOutcome.Checked, sliced.toString())
        assertEquals(setOf(path), sliced.compiled)
        assertEquals(full.verdictOf(path).key(), sliced.result.verdictOf(path).key())
    }
}
