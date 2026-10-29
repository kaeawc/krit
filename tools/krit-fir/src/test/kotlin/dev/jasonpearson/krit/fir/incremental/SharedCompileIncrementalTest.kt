package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.buildCheckResponse
import dev.jasonpearson.krit.fir.oracle.OracleResponse
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import dev.jasonpearson.krit.fir.runOneShot
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * #880: with the incremental planner on, the shared oracle/check compile (`analyzeWithCheck`) records the state,
 * builds the snapshot and persists the bundle, so default scans (where the check rides the oracle compile) get warm
 * incremental checks, in the same process and after a restart. The oracle's facts do not change.
 */
class SharedCompileIncrementalTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val root get() = tmp.resolve("repo/.krit/fir-incremental").toFile()
    private val rules = listOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() {
        IncrementalTestHook.onStage = null
        sessions.forEach { it.dispose() }
    }

    private fun write(rel: String, text: String) = src.writeSource(rel, text)
    private fun paths() = kotlinFilesIn(src)

    private fun probeModule() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        write("Other.kt", "package p\n\nfun o() { protocolProbe() }\n")
        write("Third.kt", "package p\n\nfun t(): Int = base() + 1\n")
    }

    private fun session(planning: Boolean = true): AnalysisSession =
        AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = planning, incrementalPlanning = planning)
            .also { it.snapshotRootOverride = snaps; sessions += it }

    private fun request(incrementalDir: String? = root.path) = CheckRequest(
        id = 5, command = "check", files = paths().map { FileRef(it) }, rules = rules, incrementalDir = incrementalDir,
    )

    private fun AnalysisSession.shared() = analyzeWithCheck(paths(), request())

    private fun fullCheck(): BatchResult = plainSession(src).also { sessions += it }.check(99, paths().map { FileRef(it) }, rules.toSet())

    private fun assertSameAsFull(got: BatchResult) {
        val want = fullCheck()
        for (f in paths()) assertEquals(want.verdictOf(f), got.verdictOf(f), f)
        assertEquals(want.errorFiles, got.errorFiles)
        assertEquals(want.succeeded, got.succeeded)
    }

    private fun oracleFacts(session: AnalysisSession) = OracleResponse.buildOneShot(session.analyzeFull(paths()).result)

    @Test fun aColdSharedCompileRecordsSnapshotsAndPersists() {
        probeModule()
        val stages = ArrayList<IncrementalStage>()
        IncrementalTestHook.onStage = { stages += it }
        val s = session()
        val (outcome, checked) = s.shared()

        assertNotNull(checked)
        assertEquals("full", checked.plan?.path)
        assertEquals("cold", checked.plan?.reason)
        assertSameAsFull(checked)
        // One compile: the frontend shared by the oracle and the check, then the snapshot backend.
        assertEquals(listOf(IncrementalStage.SNAPSHOT_BACKEND), stages.filter { it == IncrementalStage.SNAPSHOT_BACKEND })
        assertNotNull(s.incrementalState.compileKey, "the shared compile records the state")
        assertNull(s.sliceFallback(), "and builds the snapshot")
        // The oracle facts are those of a compile without recording or snapshot.
        assertEquals(oracleFacts(session(planning = false)), OracleResponse.buildOneShot(outcome.result))

        s.flushPersistence()
        assertEquals(1, root.listFiles()!!.count { it.isDirectory }, "the bundle is persisted: ${root.list()?.toList()}")
        // The response carries what the planner did.
        assertTrue("\"incremental\":{\"path\":\"full\",\"reason\":\"cold\"" in buildCheckResponse(checked), buildCheckResponse(checked))
    }

    @Test fun theNextSharedRequestAfterAnEditIsServedIncrementally() {
        probeModule()
        val s = session()
        s.shared()
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); protocolProbe() }\n")

        val (outcome, checked) = s.shared()
        assertNotNull(checked)
        assertEquals("incremental", checked.plan?.path, "plan: ${checked.plan}")
        assertSameAsFull(checked)
        // The oracle still gets the whole module's facts, as from its own compile.
        assertEquals(oracleFacts(session(planning = false)), OracleResponse.buildOneShot(outcome.result))
    }

    @Test fun aRestartedProcessStartsWarmFromTheBundleTheSharedCompileWrote() {
        probeModule()
        val first = session()
        first.shared()
        first.flushPersistence()
        first.dispose()

        write("Other.kt", "package p\n\nfun o() { protocolProbe(); protocolProbe() }\n")
        val restarted = session()
        val (_, checked) = restarted.shared()
        assertNotNull(checked)
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "load: ${restarted.persistence!!.lastLoad}")
        assertEquals("incremental", checked.plan?.path, "plan: ${checked.plan}")
        assertSameAsFull(checked)
    }

    @Test fun aOneShotProcessWritesTheBundleBeforeItsOutput() {
        probeModule()
        val requestFile = tmp.resolve("request.json").toFile()
        requestFile.writeText(
            """{"id":3,"command":"check","files":[${paths().joinToString(",") { "{\"path\":\"$it\"}" }}],""" +
                """"rules":["ProtocolProbe"],"incrementalDir":"${root.path}"}""",
        )
        val checkOut = tmp.resolve("check.json").toFile()
        runOneShot(listOf(src.path), tmp.resolve("types.json").toString(), null, listOf(testStdlib), cacheDepsOutPath = null,
            checkRequestPath = requestFile.path, checkOutPath = checkOut.path, incremental = true, snapshotRoot = snaps)

        assertTrue("\"incremental\":{\"path\":\"full\",\"reason\":\"cold\"" in checkOut.readText(), checkOut.readText())
        assertEquals(1, root.listFiles()!!.count { it.isDirectory }, "the one-shot run persisted its bundle")
        // The next process starts warm from it.
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        val (_, checked) = session().shared()
        assertEquals("incremental", checked?.plan?.path, "plan: ${checked?.plan}")
        assertSameAsFull(checked!!)
    }

    @Test fun withoutAnIncrementalDirNothingIsPersistedButTheSessionStillWarms() {
        probeModule()
        val s = session()
        s.analyzeWithCheck(paths(), request(incrementalDir = null))
        s.flushPersistence()
        assertTrue(!root.exists() || root.listFiles()!!.none { it.isDirectory })
        write("Third.kt", "package p\n\nfun t(): Int = base() + 2\n")
        val (_, checked) = s.analyzeWithCheck(paths(), request(incrementalDir = null))
        assertEquals("incremental", checked?.plan?.path)
    }

    @Test fun withThePlannerOffTheSharedCompileStaysUnrecorded() {
        probeModule()
        val s = session(planning = false)
        val (_, checked) = s.shared()
        assertNotNull(checked)
        assertNull(checked.plan)
        assertNull(s.incrementalState.compileKey)
    }
}
