package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * The persisted bundle (#878) against a fresh full check, on the planner differential's synthetic module and edit run
 * (body, signature, overload, add, delete, sealed, shadow, rename and the Serializable chain, which moves the set of
 * names reaching `Serializable`). Every third step the planned session is replaced by a new one on the same bundle
 * root, as a daemon restart or a new one-shot `krit` run would, with one to three edits made while nothing was
 * running. The new session's first check, and every check after it, must equal a fresh full check by a plain session
 * with no incremental state, file by file: findings, compiler-error gate and rule errors. That first check must also be
 * served from the loaded bundle: incrementally, or by a documented fallback (never COLD).
 *
 * A second run restarts after every step, with edits that delete files, rename them, remove sealed subclasses and move
 * signatures: the state that is persisted is then the product of a slice merge that deleted files or changed surfaces
 * (`removed`, `binaryStale` and the snapshot's surface hashes are not empty), not of a full check.
 *
 * Tagged [IncrementalPlannerDifferentialTest.TAG]: it runs in `incrementalPlannerTest`, not in `test`.
 */
@Tag(IncrementalPlannerDifferentialTest.TAG)
class PersistedBundleDifferentialTest {
    companion object {
        private val FILES = System.getProperty("krit.fir.planner.files")?.toInt() ?: 400
        private val EDITS = System.getProperty("krit.fir.planner.edits")?.toInt() ?: 45

        private val CYCLE = listOf(
            "serAdd", "serBody", "serNested", "serRename", "serReparent", "body", "serErr", "serDelete",
            "serNested", "serReparent", "serRename", "serBody", "serAdd", "serErr", "serDelete", "serBody",
            "body", "sig", "add", "overload", "comment", "sealed", "delete", "rename", "renameBreak", "renameRepair",
            "shadow", "redeclare", "abstractBreak", "abstractRepair", "multi", "deleteBreak", "deleteRepair", "sig", "redeclare", "multi",
            "body", "add", "redeclare", "comment", "sealed", "multi", "abstractBreak", "abstractRepair", "overload", "shadow",
            "redeclare", "delete", "rename", "sig", "multi", "body", "redeclare", "sealed", "comment", "body",
        )
        // Surface changes, deletions and renames, one after the other: every restart follows a slice merge of one.
        private val MERGE_HEAVY_CYCLE = listOf(
            "delete", "sealed", "rename", "sig", "deleteBreak", "deleteRepair", "renameBreak", "renameRepair", "redeclare",
            "abstractBreak", "abstractRepair", "overload", "delete", "shadow", "sealed", "add", "multi", "delete",
        )
        private val ALLOWED_FALLBACKS = setOf(
            FullReason.NEEDS_RESNAPSHOT, FullReason.THRESHOLD, FullReason.PACKAGE_REMOVED, FullReason.SERIALIZABLE_NAMES_CHANGED,
            FullReason.SLICE_UNAVAILABLE, FullReason.GLOBAL_COMPILER_ERROR,
        )
    }

    @TempDir lateinit var tmp: Path

    @Test fun restartedSessionsEqualFreshFullChecksAcrossAMixedEditRun() =
        drive(restartEvery = 3, cycle = CYCLE, edits = EDITS, minLoadedIncremental = 7)

    @Test fun restartingAfterEverySliceMergeOfDeletionsAndSurfaceChangesEqualsFreshFullChecks() =
        drive(restartEvery = 1, cycle = MERGE_HEAVY_CYCLE, edits = minOf(EDITS, 30), minLoadedIncremental = 5)

    private fun drive(restartEvery: Int, cycle: List<String>, edits: Int, minLoadedIncremental: Int) {
        val src = tmp.resolve("src").toFile()
        val project = SyntheticProject(src, FILES).also { it.generate() }
        val root = tmp.resolve("fir-incremental").toFile()
        val rules = FirRuleDiscovery.rules.map { it.ruleId }.toSet()

        fun newSession() = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); it.configurePersistence(root) }

        var planned = newSession()
        val reference = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = false, incrementalPlanning = false)

        var edit = 0
        var cursor = 0
        var staleVerdicts = 0
        var restarts = 0
        var loads = 0
        var loadedIncremental = 0
        var checks = 0
        var checksIncremental = 0
        val log = StringBuilder()
        val fallbacks = ArrayList<String>()
        var restartMillis = 0L

        fun step(label: String, afterRestart: Boolean, expectLoad: Boolean) {
            val all = project.files().sorted()
            val supply = edit % 2 == 0
            val files = all.map { FileRef(it, if (supply) sha1Hex(File(it).readBytes()) else "") }
            val t0 = System.nanoTime()
            val got = planned.check(edit.toLong(), files, emptySet())
            val millis = (System.nanoTime() - t0) / 1_000_000
            val want = reference.check(edit.toLong(), files, rules)
            val stale = compare(files.map { it.path }, want, got)
            staleVerdicts += stale.size
            val plan = assertNotNull(planned.lastPlan)
            val load = planned.persistence?.lastLoad
            checks++
            if (plan.path == PlanPath.INCREMENTAL) checksIncremental++
            if (afterRestart) {
                restarts++
                restartMillis += millis
                if (load is BundleLoad.Loaded) {
                    loads++
                    if (plan.path == PlanPath.INCREMENTAL) loadedIncremental++
                    else {
                        fallbacks += "e$edit $label: ${plan.reason} ${plan.detail}"
                        assertTrue(plan.reason in ALLOWED_FALLBACKS, "e$edit $label: a loaded bundle fell back with ${plan.reason} ${plan.detail}\n$log")
                    }
                } else {
                    assertTrue(!expectLoad, "e$edit $label: the bundle should have loaded but: $load\n$log")
                }
            }
            log.append("e$edit %-12s %-7s %-11s %5d ms dirty=%3d changed=%d new=%d del=%d reason=%s load=%s stale=%d%n".format(
                label, if (afterRestart) "restart" else "", plan.path, millis, plan.dirtyFiles, plan.changedFiles, plan.newFiles,
                plan.deletedFiles, plan.reason, load?.let { if (it is BundleLoad.Loaded) "Loaded" else "Miss" } ?: "-", stale.size))
            assertTrue(stale.isEmpty(), "edit $edit ($label) left ${stale.size} stale verdicts: ${stale.take(5)}\n$log")
            edit++
        }

        step("cold", afterRestart = false, expectLoad = false)
        while (edit <= edits) {
            val restart = edit % restartEvery == 0
            val count = if (restart) 1 + (edit / restartEvery) % 3 else 1
            val kinds = (0 until count).map { cycle[cursor++ % cycle.size] }
            for (kind in kinds) project.apply(kind)
            var expectLoad = false
            if (restart) {
                // A state that has a snapshot and verdicts in step is persisted by the flush; a module with a compile
                // error has none, and the restart finds an older bundle or none.
                val snapshotted = planned.incrementalState.snapshot != null &&
                    planned.verdicts.generation == planned.incrementalState.generation
                planned.flushPersistence()
                planned.dispose()
                planned = newSession()
                expectLoad = snapshotted
            }
            step(kinds.joinToString("+"), afterRestart = restart, expectLoad = expectLoad)
        }
        println(log)
        println("restarts=$restarts loaded=$loads loadedAndIncremental=$loadedIncremental checks=$checks incremental=$checksIncremental " +
            "restart-check total=$restartMillis ms stale=$staleVerdicts fallbacks=$fallbacks")
        assertEquals(0, staleVerdicts)
        assertTrue(restarts >= 10, "the run restarted $restarts times")
        assertTrue(loads >= restarts * 7 / 10, "only $loads of $restarts restarts loaded a bundle\n$log")
        assertTrue(loadedIncremental >= loads * minLoadedIncremental / 10, "only $loadedIncremental of $loads loaded sessions served their first check incrementally: $fallbacks\n$log")
        planned.dispose()
        reference.dispose()
    }

    /** Files whose planned verdict differs from the full check's. */
    private fun compare(paths: List<String>, want: BatchResult, got: BatchResult): List<String> {
        val out = ArrayList<String>()
        fun findings(r: BatchResult, p: String) = r.findings.filter { it.path == p }.map { "${it.rule}@${it.line}:${it.col} ${it.message}" }.sorted()
        for (p in paths) {
            if (findings(want, p) != findings(got, p)) out += "$p findings want=${findings(want, p)} got=${findings(got, p)}"
            if (want.errorFiles[p] != got.errorFiles[p]) out += "$p error want=${want.errorFiles[p]} got=${got.errorFiles[p]}"
            val wantRuleErrors = want.ruleErrors.filterValues { p in it }.mapValues { it.value[p] }
            val gotRuleErrors = got.ruleErrors.filterValues { p in it }.mapValues { it.value[p] }
            if (wantRuleErrors != gotRuleErrors) out += "$p ruleErrors want=$wantRuleErrors got=$gotRuleErrors"
        }
        if (want.succeeded != got.succeeded) out += "succeeded want=${want.succeeded} got=${got.succeeded}"
        if (want.skipped != got.skipped) out += "skipped want=${want.skipped} got=${got.skipped}"
        if (want.rules != got.rules) out += "rules differ"
        if (want.crashed != got.crashed) out += "crashed differ"
        if ((want.firstCompilerError == null) != (got.firstCompilerError == null)) {
            out += "firstCompilerError want=${want.firstCompilerError} got=${got.firstCompilerError}"
        }
        if (want.findings.size != got.findings.size) out += "finding count want=${want.findings.size} got=${got.findings.size}"
        return out
    }
}
