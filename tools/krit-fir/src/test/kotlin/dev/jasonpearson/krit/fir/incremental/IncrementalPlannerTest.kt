package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.checkers.protocol.TieredProbe
import dev.jasonpearson.krit.fir.SdkLevels
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
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** The dirty-set planner and the verdict cache behind `AnalysisSession.check` (#872). */
class IncrementalPlannerTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val rules = setOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() {
        IncrementalTestHook.onStage = null
        sessions.forEach { it.dispose() }
    }

    private fun write(rel: String, text: String) = src.writeSource(rel, text)
    private fun path(rel: String) = canon(File(src, rel).path)
    private fun refs() = kotlinFilesIn(src).map { FileRef(it) }

    private fun planned(classpath: List<String> = listOf(testStdlib), planning: Boolean = true): AnalysisSession =
        AnalysisSession(listOf(src.path), classpath, incrementalRecording = true, incrementalPlanning = planning)
            .also { it.snapshotRootOverride = snaps; sessions += it }

    private fun fullCheck(
        files: List<FileRef> = refs(), ruleIds: Set<String> = rules, configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(), scanPaths: Map<String, String> = emptyMap(), sdk: Map<String, SdkLevels> = emptyMap(),
    ): BatchResult = plainSession(src).also { sessions += it }.check(99, files, ruleIds, configs, testFiles, scanPaths, sdk)

    /** The planned result must be the full check's, file by file. */
    private fun assertSameAsFull(got: BatchResult, want: BatchResult, files: List<String> = kotlinFilesIn(src)) {
        for (f in files) assertEquals(want.verdictOf(f), got.verdictOf(f), f)
        assertEquals(want.errorFiles, got.errorFiles)
        assertEquals(want.succeeded, got.succeeded)
        assertEquals(want.skipped, got.skipped)
        assertEquals(want.rules, got.rules)
        assertEquals(want.findings.size, got.findings.size)
        assertEquals(want.firstCompilerError == null, got.firstCompilerError == null)
    }

    private fun probeModule() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        write("Other.kt", "package p\n\nfun o() { protocolProbe() }\n")
    }

    private fun AnalysisSession.checkAll(
        files: List<FileRef> = refs(), ruleIds: Set<String> = rules, configs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(), scanPaths: Map<String, String> = emptyMap(), sdk: Map<String, SdkLevels> = emptyMap(),
    ) = check(1, files, ruleIds, configs, testFiles, scanPaths, sdk)

    private val AnalysisSession.plan get() = assertNotNull(lastPlan)

    // ---- the flag --------------------------------------------------------------------------------------------

    @Test fun withThePlannerOffEveryCheckIsAFullCompileAndNothingIsPlanned() {
        probeModule()
        val session = planned(planning = false)
        assertFalse(session.plannerEnabled)
        session.checkAll()
        val generation = session.incrementalState.generation
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        val result = session.checkAll()
        assertNull(session.lastPlan)
        assertEquals(generation + 1, session.incrementalState.generation, "a whole-module commit replaced the state")
        assertEquals(0, session.verdicts.size, "no verdict is kept")
        assertSameAsFull(result, fullCheck())
    }

    @Test fun thePlannerNeedsRecordingAndSnapshots() {
        probeModule()
        val noRecording = AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = false, incrementalPlanning = true)
        assertFalse(noRecording.plannerEnabled)
        val noSnapshots = AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalSnapshot = false, incrementalPlanning = true)
        assertFalse(noSnapshots.plannerEnabled)
        assertTrue(planned().plannerEnabled)
    }

    // ---- the happy path ------------------------------------------------------------------------------------------

    @Test fun aColdRequestIsAFullCheckThatBuildsTheSnapshotAndEveryRequestedVerdict() {
        probeModule()
        val session = planned()
        val result = session.checkAll()
        assertEquals(PlanPath.FULL, session.plan.path)
        assertEquals(FullReason.COLD, session.plan.reason)
        assertNotNull(session.incrementalState.snapshot)
        assertEquals(3, session.verdicts.size)
        assertEquals(session.incrementalState.generation, session.verdicts.generation)
        assertSameAsFull(result, fullCheck())
        assertTrue(result.findings.isNotEmpty())
    }

    @Test fun aBodyEditCompilesOnlyTheEditedFileAndTheRestComesFromTheVerdictCache() {
        probeModule()
        val session = planned()
        session.checkAll()
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        val compiled = ArrayList<IncrementalStage>()
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE) compiled += it }
        val result = session.checkAll()
        with(session.plan) {
            assertEquals(PlanPath.INCREMENTAL, path)
            assertNull(reason)
            assertEquals(1, changedFiles); assertEquals(0, newFiles); assertEquals(0, deletedFiles)
            assertEquals(1, dirtyFiles); assertEquals(1, waves); assertEquals(1, waveMillis.size)
        }
        assertEquals(listOf(IncrementalStage.SLICE_COMPILE), compiled)
        assertSameAsFull(result, fullCheck())
    }

    @Test fun aRequestWithNothingChangedCompilesNothing() {
        probeModule()
        val session = planned()
        val first = session.checkAll()
        var compiles = 0
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE) compiles++ }
        val second = session.checkAll()
        assertEquals(0, compiles)
        with(session.plan) { assertEquals(PlanPath.INCREMENTAL, path); assertEquals(0, waves); assertEquals(0, dirtyFiles) }
        assertSameAsFull(second, first)
    }

    @Test fun aSuppliedHashOfAnotherSchemeForUnchangedContentCompilesNothing() {
        probeModule()
        val session = planned()
        session.checkAll(kotlinFilesIn(src).map { FileRef(it, "go-hash-1") })
        var compiles = 0
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE) compiles++ }
        val result = session.checkAll(kotlinFilesIn(src).map { FileRef(it, "other-scheme") })
        assertEquals(0, compiles)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(0, session.plan.changedFiles)
        assertSameAsFull(result, fullCheck())
    }

    @Test fun aSuppliedHashThatMatchesTheLastOneIsTrustedWithoutReadingTheFile() {
        probeModule()
        val session = planned()
        val files = kotlinFilesIn(src).map { FileRef(it, "h-" + File(it).name) }
        session.checkAll(files)
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        session.checkAll(files)
        assertEquals(0, session.plan.changedFiles, "the request says nothing changed")
        session.checkAll(kotlinFilesIn(src).map { FileRef(it) })
        assertEquals(1, session.plan.changedFiles, "without a hash the content is compared")
    }

    @Test fun aSignatureEditCompilesItsDependentsInAFollowUpWave() {
        probeModule()
        write("Far.kt", "package q\n\nimport p.base\n\nfun far() = base() + 1\n")
        write("Unrelated.kt", "package r\n\nfun unrelated() = 1\n")
        val session = planned()
        session.checkAll()
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Long = 1L\n")
        val result = session.checkAll()
        with(session.plan) {
            assertEquals(PlanPath.INCREMENTAL, path)
            assertTrue(waves >= 2, "waves=$waves")
            assertTrue(dirtyFiles >= 3, "Api.kt and its two users: $dirtyFiles")
            assertTrue(dirtyFiles < 5, "Unrelated.kt and Other.kt are not touched: $dirtyFiles")
        }
        assertSameAsFull(result, fullCheck())
    }

    @Test fun aNewFileAndADeletedFileAreServedIncrementally() {
        probeModule()
        val session = planned()
        session.checkAll()
        write("Added.kt", "package p\n\nfun added() { protocolProbe() }\n")
        var result = session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(1, session.plan.newFiles)
        assertSameAsFull(result, fullCheck())
        File(path("Other.kt")).delete()
        result = session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(1, session.plan.deletedFiles)
        assertSameAsFull(result, fullCheck())
        // A deleted declaration breaks its users, who are compiled.
        File(path("Api.kt")).delete()
        result = session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertTrue(result.errorFiles.isNotEmpty())
        assertSameAsFull(result, fullCheck())
    }

    // ---- structural rules ------------------------------------------------------------------------------------------

    @Test fun aRedeclarationInAnotherFileIsSeenOnBothSides() {
        write("A.kt", "package p\n\nfun dup(): Int = 1\nfun a() { dup() }\n")
        write("B.kt", "package p\n\nfun b() = 2\n")
        val session = planned()
        session.checkAll()
        write("B.kt", "package p\n\nfun b() = 2\nfun dup(): Int = 3\n")
        var result = session.checkAll()
        assertTrue(path("A.kt") in result.errorFiles, "the earlier declaration is in conflict too: ${result.errorFiles}")
        assertSameAsFull(result, fullCheck())
        write("B.kt", "package p\n\nfun b() = 2\n")
        result = session.checkAll()
        assertTrue(result.errorFiles.isEmpty(), result.errorFiles.toString())
        assertSameAsFull(result, fullCheck())
    }

    @Test fun anImplementerOfARenamedAbstractMemberIsCompiled() {
        write("Base.kt", "package p\n\nabstract class Base { abstract fun m(): Int }\n")
        write("Impl.kt", "package p\n\nclass Impl : Base() { override fun m(): Int = 1 }\n")
        write("Other.kt", "package p\n\nfun o() = 1\n")
        val session = planned()
        session.checkAll()
        write("Base.kt", "package p\n\nabstract class Base { abstract fun m2(): Int }\n")
        val result = session.checkAll()
        assertTrue(path("Impl.kt") in result.errorFiles, result.errorFiles.toString())
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertSameAsFull(result, fullCheck())
    }

    @Test fun aCallThroughASubclassFindsAChangedMemberOfItsSuperclass() {
        write("Base.kt", "package p\n\nopen class Base { fun member(): Int = 1 }\n")
        write("Sub.kt", "package p\n\nclass Sub : Base()\n")
        write("Use.kt", "package p\n\nfun use() = Sub().member()\n")
        write("Other.kt", "package p\n\nfun o() = 1\n")
        val session = planned()
        session.checkAll()
        write("Base.kt", "package p\n\nopen class Base { fun member2(): Int = 1 }\n")
        val result = session.checkAll()
        assertTrue(path("Use.kt") in result.errorFiles, result.errorFiles.toString())
        assertSameAsFull(result, fullCheck())
    }

    @Test fun aSealedSubclassAddedInAnotherFileUpdatesTheExhaustiveWhen() {
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\n")
        write("W.kt", "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n")
        write("Other.kt", "package p\n\nfun o() = 1\n")
        val session = planned()
        session.checkAll()
        write("D2.kt", "package p\n\nclass D2 : U\n")
        val result = session.checkAll()
        assertTrue(path("W.kt") in result.errorFiles, "the when is no longer exhaustive: ${result.errorFiles}")
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(session.plan.addedByStaleness + session.plan.addedBySealedClosure > 0, true)
        assertSameAsFull(result, fullCheck())
    }

    // ---- the verdict cache: keys ---------------------------------------------------------------------------------------

    @Test fun aRuleConfigChangeDoesNotReuseVerdicts() {
        probeModule()
        val session = planned()
        val one = mapOf("ProtocolProbe" to mapOf<String, Any?>("tag" to "one"))
        val two = mapOf("ProtocolProbe" to mapOf<String, Any?>("tag" to "two"))
        val first = session.checkAll(configs = one)
        assertTrue(first.findings.all { "configured: one" in it.message }, first.findings.toString())
        val second = session.checkAll(configs = two)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(3, session.plan.unverdictedFiles, "every verdict was computed under another config")
        assertTrue(second.findings.isNotEmpty() && second.findings.all { "configured: two" in it.message }, second.findings.toString())
        assertSameAsFull(second, fullCheck(configs = two))
        // Same config again: reused.
        session.checkAll(configs = two)
        assertEquals(0, session.plan.unverdictedFiles)
        assertEquals(0, session.plan.waves)
    }

    @Test fun anEnabledRuleChangeDoesNotReuseVerdicts() {
        probeModule()
        val session = planned()
        session.checkAll(ruleIds = setOf("ProtocolProbe"))
        val both = session.checkAll(ruleIds = setOf("ProtocolProbe", "ThrowingProbe"))
        assertEquals(3, session.plan.unverdictedFiles)
        assertSameAsFull(both, fullCheck(ruleIds = setOf("ProtocolProbe", "ThrowingProbe")))
    }

    @Test fun aTestFileFlagChangeRecomputesThatFileOnly() {
        probeModule()
        val session = planned()
        val configs = mapOf("ProtocolProbe" to mapOf<String, Any?>("tag" to "t"))
        val before = session.checkAll(configs = configs)
        assertTrue(before.findings.none { "(test file)" in it.message })
        val use = path("Use.kt")
        val after = session.checkAll(configs = configs, testFiles = setOf(use))
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(1, session.plan.unverdictedFiles, "only the file whose flag changed")
        assertTrue(after.findings.filter { it.path == use }.all { "(test file)" in it.message }, after.findings.toString())
        assertTrue(after.findings.filter { it.path != use }.none { "(test file)" in it.message })
        assertSameAsFull(after, fullCheck(configs = configs, testFiles = setOf(use)))
        // And back.
        val back = session.checkAll(configs = configs)
        assertEquals(1, session.plan.unverdictedFiles)
        assertSameAsFull(back, before)
    }

    @Test fun aScanPathOrSdkLevelChangeRecomputesThatFileOnly() {
        probeModule()
        val session = planned()
        val configs = mapOf("ProtocolProbe" to mapOf<String, Any?>("showScanPath" to true, "showSdk" to true))
        session.checkAll(configs = configs)
        val use = path("Use.kt")
        val scan = session.checkAll(configs = configs, scanPaths = mapOf(use to "rel/Use.kt"))
        assertEquals(1, session.plan.unverdictedFiles)
        assertTrue(scan.findings.single { it.path == use }.message.contains("scan=rel/Use.kt"), scan.findings.toString())
        val sdk = mapOf(use to SdkLevels(21, 34))
        val levels = session.checkAll(configs = configs, scanPaths = mapOf(use to "rel/Use.kt"), sdk = sdk)
        assertEquals(1, session.plan.unverdictedFiles)
        assertTrue(levels.findings.single { it.path == use }.message.contains("sdk=21/34"), levels.findings.toString())
        assertSameAsFull(levels, fullCheck(configs = configs, scanPaths = mapOf(use to "rel/Use.kt"), sdk = sdk))
    }

    @Test fun theKeysSeparateEveryInputAndIgnoreSpelling() {
        val file = path("A.kt").also { write("A.kt", "package p\n") }
        val base = VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), emptySet(), emptyMap(), emptyMap()).of(file)
        assertEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), emptySet(), emptyMap(), emptyMap()).of(file))
        assertNotEquals(base, VerdictKeys(setOf("R", "S"), mapOf("R" to mapOf<String, Any?>("a" to 1)), emptySet(), emptyMap(), emptyMap()).of(file))
        assertNotEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 2)), emptySet(), emptyMap(), emptyMap()).of(file))
        assertNotEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), setOf(file), emptyMap(), emptyMap()).of(file))
        assertNotEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), emptySet(), mapOf(file to "x"), emptyMap()).of(file))
        assertNotEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), emptySet(), emptyMap(), mapOf(file to SdkLevels(1, 2))).of(file))
        // Another file's flag is not this file's.
        val other = path("B.kt")
        assertEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), setOf(other), mapOf(other to "x"), mapOf(other to SdkLevels(1, 2))).of(file))
        // A non-canonical spelling of the same file.
        val spelled = File(src, "../src/A.kt").path
        assertNotEquals(base, VerdictKeys(setOf("R"), mapOf("R" to mapOf<String, Any?>("a" to 1)), setOf(spelled), emptyMap(), emptyMap()).of(file))
        assertEquals(
            VerdictKeys(setOf("R"), emptyMap(), setOf(spelled), emptyMap(), emptyMap()).of(file),
            VerdictKeys(setOf("R"), emptyMap(), setOf(file), emptyMap(), emptyMap()).of(file),
        )
    }

    // ---- tiered rules ---------------------------------------------------------------------------------------------

    // No built-in rule is tiered, so these tests arm the test-only TieredProbe (a rule with no checkers).
    private val tieredRule get() = TieredProbe.ruleId

    @Test fun theDefaultPolicyUsesTheFullCompileWheneverATieredRuleIsEnabled() = TieredProbe.armed {
        probeModule()
        val session = planned()
        session.checkAll(ruleIds = rules + tieredRule)
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        val result = session.checkAll(ruleIds = rules + tieredRule)
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.TIERED_RULES, reason); assertTrue(detail!!.contains(tieredRule)) }
        assertSameAsFull(result, fullCheck(ruleIds = rules + tieredRule))
        // "All rules" includes the tiered ones.
        session.checkAll(ruleIds = emptySet())
        assertEquals(FullReason.TIERED_RULES, session.plan.reason)
        // Without a tiered rule the same session serves incrementally.
        session.checkAll(ruleIds = rules)
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        session.checkAll(ruleIds = rules)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun aTieredFallbackIsAPlainCheckThatLeavesTheStateAlone() = TieredProbe.armed {
        probeModule()
        val session = planned()
        val tiered = rules + tieredRule
        val first = session.checkAll(ruleIds = tiered)
        assertEquals(FullReason.TIERED_RULES, session.plan.reason)
        assertNull(session.incrementalState.compileKey, "no recording")
        assertNull(session.incrementalState.snapshot, "no snapshot is built")
        assertEquals(0, session.incrementalState.generation)
        assertEquals(0, session.verdicts.size)
        assertEquals(0, session.fullCommits)
        assertSameAsFull(first, fullCheck(ruleIds = tiered))
        // With a state from an earlier request, a tiered request does not change it either.
        session.checkAll(ruleIds = rules)
        val generation = session.incrementalState.generation
        val snapshot = session.incrementalState.snapshot
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        val second = session.checkAll(ruleIds = tiered)
        assertEquals(FullReason.TIERED_RULES, session.plan.reason)
        assertEquals(generation, session.incrementalState.generation)
        assertTrue(snapshot === session.incrementalState.snapshot)
        assertSameAsFull(second, fullCheck(ruleIds = tiered))
        // The state still diffs correctly against what changed in between.
        val third = session.checkAll(ruleIds = rules)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(1, session.plan.changedFiles)
        assertSameAsFull(third, fullCheck())
    }

    @Test fun excludeTieredRunsOnlyTheOthersAndStaysIncremental() = TieredProbe.armed {
        probeModule()
        val session = planned()
        session.tieredRulePolicy = BuiltInTieredPolicy.EXCLUDE_TIERED
        val requested = rules + tieredRule
        val cold = session.checkAll(ruleIds = requested)
        assertEquals(rules.toList(), cold.rules, "the tiered rule is disabled")
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        val result = session.checkAll(ruleIds = requested)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(rules.toList(), result.rules)
        assertSameAsFull(result, fullCheck(ruleIds = rules))
    }

    @Test fun theTieredPolicyIsASeamAndAFailingPolicyIsAFallback() {
        probeModule()
        val session = planned()
        session.checkAll()
        val seen = ArrayList<Pair<Set<String>, Set<String>>>()
        session.tieredRulePolicy = TieredRulePolicy { enabled, tiered -> seen += enabled to tiered; TieredDecision.RunIncrementally(enabled) }
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(1, seen.size)
        assertEquals(emptySet(), seen.single().second, "no built-in rule is tiered")
        TieredProbe.armed {
            // The policy sees the tiered ids of the moment: the armed probe.
            write("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
            session.checkAll()
            assertEquals(setOf(TieredProbe.ruleId), seen.last().second)
        }
        session.tieredRulePolicy = TieredRulePolicy { _, _ -> error("policy boom") }
        val result = session.checkAll()
        assertEquals(FullReason.TIERED_RULES, session.plan.reason)
        assertTrue(session.plan.detail!!.contains("policy boom"))
        assertSameAsFull(result, fullCheck())
    }

    // ---- fallbacks ---------------------------------------------------------------------------------------------------

    @Test fun aModuleWithACompilerErrorHasNoSnapshotSoEveryRequestIsAFullCheck() {
        probeModule()
        write("Broken.kt", "package p\n\nfun broken() { missing() }\n")
        val session = planned()
        session.checkAll()
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.SLICE_UNAVAILABLE, reason); assertEquals("NO_SNAPSHOT", detail) }
        assertSameAsFull(result, fullCheck())
        // Once the module is clean, the full check that serves the next request builds the snapshot again.
        write("Broken.kt", "package p\n\nfun broken() {}\n")
        session.checkAll()
        assertEquals(PlanPath.FULL, session.plan.path)
        assertNotNull(session.incrementalState.snapshot)
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun aChangedClasspathIsAFallback() {
        probeModule()
        val extra = tmp.resolve("extra-classes").toFile().apply { mkdirs() }
        val session = planned(listOf(testStdlib, extra.path))
        session.checkAll()
        File(extra, "x.txt").writeText("changed")
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.SLICE_UNAVAILABLE, reason); assertEquals("STATE_CHANGED", detail) }
        assertSameAsFull(result, fullCheck())
        // The full check recorded the new classpath: incremental again.
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun anExceptionInThePlannerIsAFallbackWithTheCorrectResult() {
        probeModule()
        val session = planned()
        session.checkAll()
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        IncrementalTestHook.onStage = { if (it == IncrementalStage.PLAN) error("planner boom") }
        val result = session.checkAll()
        IncrementalTestHook.onStage = null
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.EXCEPTION, reason); assertTrue(detail!!.contains("planner boom")) }
        assertSameAsFull(result, fullCheck())
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun expectActualInTheSourcesIsAFallback() {
        for ((p, t) in mapOf(
            "common/E.kt" to "package p\n\nexpect fun plat(): String\n",
            "jvm/A.kt" to "package p\n\nactual fun plat(): String = \"jvm\"\n",
            "jvm/U.kt" to "package p\n\nfun use() = plat()\n",
        )) write(p, t)
        val session = planned()
        session.checkAll()
        write("jvm/U.kt", "package p\n\nfun use() = plat() + \"x\"\n")
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.SLICE_UNAVAILABLE, reason); assertEquals("EXPECT_ACTUAL", detail) }
        assertSameAsFull(result, fullCheck())
    }

    @Test fun aFailingSliceCompileIsAFallbackWithTheCorrectResult() {
        probeModule()
        val session = planned()
        session.checkAll()
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE) error("slice boom") }
        val result = session.checkAll()
        IncrementalTestHook.onStage = null
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.SLICE_UNAVAILABLE, reason); assertTrue(detail!!.startsWith("SLICE_FAILED")) }
        assertSameAsFull(result, fullCheck())
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun aSliceThatFailsInALaterWaveReplacesTheHalfAppliedState() {
        probeModule()
        write("Far.kt", "package q\n\nimport p.base\n\nfun far() = base() + 1\n")
        val session = planned()
        session.checkAll()
        var compiles = 0
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE && ++compiles == 2) error("second wave boom") }
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Long = 1L\n")
        val result = session.checkAll()
        IncrementalTestHook.onStage = null
        assertEquals(PlanPath.FULL, session.plan.path)
        assertEquals(1, session.plan.waves, "the first wave ran and was merged")
        assertSameAsFull(result, fullCheck())
        assertEquals(session.incrementalState.generation, session.verdicts.generation)
        // The state is whole again: an untouched request compiles nothing and agrees.
        compiles = 0
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE) compiles++ }
        assertSameAsFull(session.checkAll(), result)
        assertEquals(0, compiles)
    }

    @Test fun staleFilesPastTheBoundMakeTheNextRequestAFullCheckThatResnapshots() {
        for (i in 0 until 40) write("F$i.kt", "package p\n\nfun f$i(): Int = $i\n")
        val session = planned()
        session.plannerLimits = session.plannerLimits.copy(minDirtyFiles = 64)
        session.checkAll(ruleIds = emptySet<String>() + "ProtocolProbe")
        for (i in 0 until 20) write("F$i.kt", "package p\n\nfun f$i(): Long = ${i}L\n")
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertTrue(session.incrementalState.binaryStale().size >= 16)
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.NEEDS_RESNAPSHOT, reason) }
        assertSameAsFull(result, fullCheck())
        assertEquals(emptySet(), session.incrementalState.binaryStale())
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun staleFilesPastTheBoundAreCarriedWhileTheModuleHasAnError() {
        for (i in 0 until 40) write("F$i.kt", "package p\n\nfun f$i(): Int = $i\n")
        val session = planned()
        session.plannerLimits = session.plannerLimits.copy(minDirtyFiles = 64)
        session.checkAll()
        for (i in 0 until 20) write("F$i.kt", "package p\n\nfun f$i(): Long = ${i}L\n")
        write("Broken.kt", "package p\n\nfun broken() { missing() }\n")
        session.checkAll()
        assertTrue(session.incrementalState.binaryStale().size >= 16)
        // A re-snapshot cannot succeed while the module has an error: the request stays incremental.
        val result = session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertSameAsFull(result, fullCheck())
        write("Broken.kt", "package p\n\nfun broken() {}\n")
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        // Clean again: now the bound applies.
        session.checkAll()
        assertEquals(FullReason.NEEDS_RESNAPSHOT, session.plan.reason)
    }

    @Test fun aDirtySetAboveTheThresholdIsAFullCheck() {
        for (i in 0 until 10) write("F$i.kt", "package p\n\nfun f$i(): Int = $i\nfun probe$i() { protocolProbe() }\n")
        write("Api.kt", "package p\n\nfun protocolProbe() {}\n")
        val session = planned()
        session.plannerLimits = PlannerLimits(maxWaves = 8, dirtyFraction = 0.2, minDirtyFiles = 1)
        session.checkAll()
        write("F1.kt", "package p\n\nfun f1(): Int = 11\n")
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        // 11 files, threshold ceil(11 * 0.2) = 3: four changed files are over it.
        for (i in 2..5) write("F$i.kt", "package p\n\nfun f$i(): Int = ${i + 100}\n")
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.THRESHOLD, reason) }
        assertSameAsFull(result, fullCheck())
        assertEquals(3, PlannerLimits(dirtyFraction = 0.2, minDirtyFiles = 1).threshold(11))
        assertEquals(64, PlannerLimits().threshold(100), "the minimum file count wins in a small module")
        assertEquals(250, PlannerLimits().threshold(1000))
    }

    @Test fun moreWavesThanTheLimitIsAFullCheck() {
        write("A.kt", "package p\n\nfun a(): Int = 1\n")
        write("B.kt", "package p\n\nfun b() = a()\n")
        write("C.kt", "package p\n\nfun c() = b()\n")
        write("D.kt", "package p\n\nfun d() = c()\n")
        val session = planned()
        session.checkAll()
        // Each return type is inferred from the next, so the change travels one file per wave.
        write("A.kt", "package p\n\nfun a(): Long = 1L\n")
        session.plannerLimits = session.plannerLimits.copy(maxWaves = 2)
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.WAVE_LIMIT, reason); assertEquals(2, waves) }
        assertSameAsFull(result, fullCheck())
        write("A.kt", "package p\n\nfun a(): Int = 1\n")
        session.plannerLimits = session.plannerLimits.copy(maxWaves = 8)
        val ok = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.INCREMENTAL, path); assertTrue(waves >= 4, "waves=$waves") }
        assertSameAsFull(ok, fullCheck())
    }

    @Test fun verdictsThatFellOutOfStepWithTheStateAreNotTrusted() {
        probeModule()
        val session = planned()
        session.checkAll()
        session.verdicts.clear()
        val result = session.checkAll()
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.STATE_LOST, reason) }
        assertSameAsFull(result, fullCheck())
        session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
    }

    @Test fun aRequestedFileThatIsNotOnDiskIsAFullCheckLikeToday() {
        probeModule()
        val session = planned()
        session.checkAll()
        val missing = File(src, "Missing.kt").path
        val files = refs() + FileRef(missing)
        val result = session.checkAll(files)
        with(session.plan) { assertEquals(PlanPath.FULL, path); assertEquals(FullReason.MISSING_VERDICT, reason) }
        assertSameAsFull(result, fullCheck(files), kotlinFilesIn(src))
    }

    @Test fun aRequestedFileWithNoVerdictIsCompiledNotAssumed() {
        probeModule()
        val session = planned()
        // The first request names two of the three files; the third has no verdict.
        session.checkAll(refs().filter { !it.path.endsWith("Other.kt") })
        val result = session.checkAll()
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertEquals(1, session.plan.unverdictedFiles)
        assertEquals(1, session.plan.dirtyFiles)
        assertTrue(result.findings.any { it.path.endsWith("Other.kt") })
        assertSameAsFull(result, fullCheck())
    }

    @Test fun theRequestsOwnSpellingOfAPathIsKept() {
        probeModule()
        val session = planned()
        val spelled = refs().map { FileRef(File(src, "../src/" + File(it.path).name).path) }
        session.checkAll(spelled)
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        val result = session.checkAll(spelled)
        assertEquals(PlanPath.INCREMENTAL, session.plan.path)
        assertTrue(result.findings.isNotEmpty() && result.findings.all { f -> spelled.any { it.path == f.path } }, result.findings.toString())
    }

    @Test fun telemetryCountsFilesAddedByStalenessAndBySealedClosure() {
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\nfun bodyOnly(): Int = 1\n")
        write("S2.kt", "package p\n\nclass D2 : U\n")
        write("Lib.kt", "package p\n\nfun lib(): Int = 1\n")
        write("Mid.kt", "package p\n\nfun mid() = lib()\n")
        val session = planned()
        session.checkAll()
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\nfun bodyOnly(): Int = 2\n")
        session.checkAll()
        with(session.plan) { assertEquals(1, addedBySealedClosure); assertEquals(0, addedByStaleness) }
        // A signature edit leaves Lib.kt and its dependent Mid.kt (whose inferred type moved) binary-stale; the next,
        // unrelated edit carries them along.
        write("Lib.kt", "package p\n\nfun lib(): Long = 1L\n")
        session.checkAll()
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\nfun bodyOnly(): Int = 3\n")
        session.checkAll()
        with(session.plan) { assertEquals(2, addedByStaleness, toString()); assertTrue(toString().contains("path=INCREMENTAL")) }
    }
}
