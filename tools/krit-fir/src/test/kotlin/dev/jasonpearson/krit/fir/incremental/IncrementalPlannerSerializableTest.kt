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
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * SerialVersionUIDInSerializableClass is not tiered: a slice gets the recorded class-name facts of the module files
 * it reads as binaries, and the planner falls back to a full check when an edit moves the set of names reaching
 * `Serializable` (#876).
 */
class IncrementalPlannerSerializableTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val rule = "SerialVersionUIDInSerializableClass"
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() {
        sessions.forEach { it.dispose() }
    }

    private fun write(rel: String, text: String) = src.writeSource(rel, text)
    private fun refs() = kotlinFilesIn(src).map { FileRef(it) }

    private fun planned(): AnalysisSession =
        AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = snaps; sessions += it }

    private fun AnalysisSession.checkRule(ruleIds: Set<String> = setOf(rule)) = check(1, refs(), ruleIds)
    private fun AnalysisSession.checkOnly(file: String, ruleIds: Set<String>) = check(1, listOf(FileRef(File(src, file).path)), ruleIds)
    private fun full(ruleIds: Set<String> = setOf(rule)): BatchResult =
        plainSession(src).also { sessions += it }.check(99, refs(), ruleIds)

    private fun BatchResult.reported() = findings.filter { it.rule == rule }.map { File(it.path).name to it.line }.sortedWith(compareBy({ it.first }, { it.second }))
    private val AnalysisSession.plan get() = assertNotNull(lastPlan)

    private fun resource(serializable: Boolean = true, error: String = "Error") = """
        package model

        sealed class Resource${if (serializable) " : java.io.Serializable" else ""} {
            companion object {
                private const val serialVersionUID = 1L
            }

            data class $error(val message: String) : Resource()
        }
    """.trimIndent() + "\n"

    private val failures = """
        package app

        class FatalFailure(msg: String) : kotlin.Error(msg)

        class Plain(msg: String) : RuntimeException(msg)
    """.trimIndent() + "\n"

    private fun module() {
        write("Resource.kt", resource())
        write("Failures.kt", failures)
        write("Other.kt", "package app\n\nfun other(): Int = 1\n")
    }

    private fun assertSameAsFull(got: BatchResult, ruleIds: Set<String> = setOf(rule)) {
        val want = full(ruleIds)
        for (f in kotlinFilesIn(src)) assertEquals(want.verdictOf(f), got.verdictOf(f), f)
        assertEquals(want.reported(), got.reported())
    }

    @Test fun theRuleIsNotTieredSoTheDefaultPolicyRunsIncrementally() {
        module()
        val session = planned()
        val cold = session.checkRule()
        assertEquals(listOf("Failures.kt" to 3, "Resource.kt" to 8), cold.reported())
        write("Other.kt", "package app\n\nfun other(): Int = 2\n")
        val result = session.checkRule()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path, "${session.plan}")
        assertSameAsFull(result)
    }

    @Test fun anEditInFailuresSeesResourceThroughTheRecordedFactsAndStaysIncremental() {
        module()
        val session = planned()
        session.checkRule()
        // Resource.kt is a binary in this slice: FatalFailure is reported only through its recorded class-name facts.
        write("Failures.kt", failures + "\nfun extra(): Int = 1\n")
        val result = session.checkRule()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path, "${session.plan}")
        assertEquals(1, session.plan.dirtyFiles)
        assertEquals(listOf("Failures.kt" to 3, "Resource.kt" to 8), result.reported())
        assertSameAsFull(result)
    }

    // Regression: with the facts withheld the slice's set lacks Resource.Error, so FatalFailure is missed.
    @Test fun withoutTheExternalFactsASliceMissesTheFinding() {
        module()
        val session = planned()
        session.checkRule()
        session.withheldExternalFacts = true
        write("Failures.kt", failures + "\nfun extra(): Int = 1\n")
        val result = session.checkRule()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertFalse(result.reported().contains("Failures.kt" to 3), "the slice read no facts of Resource.kt")
        assertTrue(full().reported().contains("Failures.kt" to 3))
    }

    @Test fun anUnrelatedBodyEditStaysIncrementalWithTheRuleEnabled() {
        module()
        val session = planned()
        session.checkRule()
        write("Other.kt", "package app\n\nfun other(): Int = 42\n")
        val result = session.checkRule()
        with(session.plan) {
            assertEquals(PlanPath.INCREMENTAL, path, "$this")
            assertNull(reason)
            assertEquals(1, dirtyFiles)
        }
        assertSameAsFull(result)
    }

    @Test fun makingTheBaseNotSerializableFallsBackAndUpdatesFailures() {
        module()
        val session = planned()
        session.checkRule()
        write("Resource.kt", resource(serializable = false))
        val result = session.checkRule()
        assertEquals(PlanPath.FULL, session.plan.path)
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason)
        assertEquals(emptyList(), result.reported())
        assertSameAsFull(result)
        // The full check replaced the state: the next small edit is incremental again.
        write("Other.kt", "package app\n\nfun other(): Int = 3\n")
        assertSameAsFull(session.checkRule())
        assertEquals(PlanPath.INCREMENTAL, session.plan.path, "${session.plan}")
    }

    @Test fun renamingTheNestedClassFallsBackAndUpdatesFailures() {
        module()
        val session = planned()
        session.checkRule()
        write("Resource.kt", resource(error = "Failure"))
        val result = session.checkRule()
        assertEquals(PlanPath.FULL, session.plan.path)
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason)
        assertEquals(listOf("Resource.kt" to 8), result.reported())
        assertSameAsFull(result)
    }

    @Test fun addingAClassThatJoinsTheSetFallsBack() {
        module()
        val session = planned()
        session.checkRule()
        write("Extra.kt", "package model\n\nclass Late : Resource()\n")
        val result = session.checkRule()
        assertSameAsFull(result)
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason, "${session.plan}")
    }

    @Test fun deletingTheFileThatHeldTheChainFallsBack() {
        module()
        val session = planned()
        session.checkRule()
        File(src, "Resource.kt").delete()
        val result = session.checkRule()
        assertSameAsFull(result)
        assertEquals(PlanPath.FULL, session.plan.path)
    }

    // The recorded facts move whatever rules a request enables, so the set check does not depend on the rule.
    @Test fun anEditThatMovesTheSetFallsBackEvenWhenTheRuleIsNotEnabled() {
        module()
        val session = planned()
        val other = setOf("ProtocolProbe")
        session.checkRule(other)
        write("Resource.kt", resource(serializable = false))
        val result = session.checkRule(other)
        assertEquals(PlanPath.FULL, session.plan.path, "${session.plan}")
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason)
        assertSameAsFull(result, other)
        // Renaming the nested class and adding a joining class move it too.
        write("Resource.kt", resource(serializable = true))
        session.checkRule(other)
        write("Resource.kt", resource(error = "Failure"))
        session.checkRule(other)
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason, "${session.plan}")
        write("Extra.kt", "package model\n\nclass Late : Resource()\n")
        session.checkRule(other)
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason, "${session.plan}")
    }

    @Test fun anEditThatDoesNotMoveTheSetStaysIncrementalWhenTheRuleIsNotEnabled() {
        module()
        val session = planned()
        val other = setOf("ProtocolProbe")
        session.checkRule(other)
        write("Other.kt", "package app\n\nfun other(): Int = 7\n")
        session.checkRule(other)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path, "${session.plan}")
        // A body edit in a file of the chain keeps the set too.
        write("Resource.kt", resource().replace("1L", "2L"))
        val result = session.checkRule(other)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path, "${session.plan}")
        assertSameAsFull(result, other)
    }

    // Regression: check with the rule, edit Resource.kt to not be Serializable and check only with the rule off, then
    // check with the rule on again. The middle request moved the recorded facts; trusting it would leave Failures.kt
    // with the verdict that still had a finding (or, in the other direction, without one).
    @Test fun ruleOnThenAnEditWithTheRuleOffThenRuleOnAgreesWithAFullCheck() {
        module()
        val session = planned()
        val other = setOf("ProtocolProbe")
        assertEquals(listOf("Failures.kt" to 3, "Resource.kt" to 8), session.checkRule().reported())
        write("Resource.kt", resource(serializable = false))
        // Only the edited file is requested, so no other file's verdict is recomputed under the rule set without the rule.
        session.checkOnly("Resource.kt", other)
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason, "${session.plan}")
        val on = session.checkRule()
        assertEquals(emptyList(), on.reported())
        assertSameAsFull(on)
    }

    // The same sequence in the direction that makes a finding appear: Resource.kt becomes Serializable while the rule
    // is off, and Failures.kt (not touched) must gain the finding.
    @Test fun aClassBecomingSerializableWhileTheRuleIsOffIsSeenWhenItIsOnAgain() {
        write("Resource.kt", resource(serializable = false))
        write("Failures.kt", failures)
        write("Other.kt", "package app\n\nfun other(): Int = 1\n")
        val session = planned()
        assertEquals(emptyList(), session.checkRule().reported())
        write("Resource.kt", resource(serializable = true))
        session.checkOnly("Resource.kt", setOf("ProtocolProbe"))
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason, "${session.plan}")
        val on = session.checkRule()
        assertEquals(listOf("Failures.kt" to 3, "Resource.kt" to 8), on.reported())
        assertSameAsFull(on)
    }

    @Test fun theSetCheckStopsTheRunRightAfterTheFirstWave() {
        module()
        val session = planned()
        session.checkRule()
        write("Resource.kt", resource(serializable = false))
        session.checkRule()
        assertEquals(FullReason.SERIALIZABLE_NAMES_CHANGED, session.plan.reason)
        assertEquals(1, session.plan.waves, "${session.plan}")
    }

    @Test fun theStateRecordsFactsPerFileAndDropsThemWithTheFile() {
        module()
        val session = planned()
        session.checkRule()
        val state = session.incrementalState
        assertTrue("Resource" in state.serializableNames() && "Error" in state.serializableNames())
        assertFalse("Plain" in state.serializableNames())
        val all = state.classFactsExcluding(emptyList())
        assertTrue(all.any { it.name == "Error" && "Resource" in it.supertypeNames })
        assertTrue(all.any { it.name == "Resource" && "Serializable" in it.supertypeNames })
        // Resource, its companion and Error.
        assertEquals(all.size - 3, state.classFactsExcluding(listOf(canon(File(src, "Resource.kt").path))).size)
    }
}
