package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
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
 * Correctness of the incremental planner (#872) against a fresh full check, on a synthetic module of about 600
 * files and a mixed run of 40 edits: body, comment, signature (return type), new overload, added and deleted
 * files, a new sealed subclass, a shadowing `println`, and a rename of a function used elsewhere (together with
 * its callers, and as a declaration-only rename that breaks them followed by the repair).
 *
 * After every edit the planned check of all files (an incremental session whose tiered rules are excluded, so the
 * incremental path engages) is compared, file by file, with a full check by a plain session that has no incremental
 * state, run with the same rules: findings, compiler-error gate and rule errors must be equal, so there is no stale
 * verdict. The run also asserts that the planner engaged, and reports how long each side took.
 *
 * It compiles the module about 40 times, so it is tagged [TAG] and runs in its own Gradle task
 * (`incrementalPlannerTest`), not in `test`.
 */
@Tag(IncrementalPlannerDifferentialTest.TAG)
class IncrementalPlannerDifferentialTest {
    companion object {
        const val TAG = "incremental-planner"
        private val FILES = System.getProperty("krit.fir.planner.files")?.toInt() ?: 600
        private val EDITS = System.getProperty("krit.fir.planner.edits")?.toInt() ?: 40

        private val CYCLE = listOf(
            "body", "sig", "add", "overload", "comment", "sealed", "delete", "rename", "renameBreak", "renameRepair",
            "shadow", "body", "deleteBreak", "deleteRepair", "sig", "shadow", "add", "comment", "sealed", "body",
        )
    }

    @TempDir lateinit var tmp: Path

    @Test fun incrementalChecksEqualFreshFullChecksAcrossAMixedEditRun() {
        val src = tmp.resolve("src").toFile()
        val project = SyntheticProject(src, FILES).also { it.generate() }
        val rules = FirRuleDiscovery.rules.map { it.ruleId }.toSet() - FirRuleDiscovery.wholeCompilationRuleIds().toSet()
        assertTrue(rules.isNotEmpty())

        val planned = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); it.tieredRulePolicy = BuiltInTieredPolicy.EXCLUDE_TIERED }
        val reference = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = false, incrementalPlanning = false)

        var edit = 0
        fun requests(supplyHashes: Boolean) = project.files().sorted().map {
            FileRef(it, if (supplyHashes) sha1Hex(File(it).readBytes()) else "")
        }

        val log = StringBuilder()
        var incrementalTotal = 0L
        var fullTotal = 0L
        var engaged = 0
        val fallbacks = ArrayList<String>()
        var staleVerdicts = 0

        fun step(label: String) {
            // Alternate the request shape: with and without the content hashes Go supplies.
            val files = requests(supplyHashes = edit % 2 == 0)
            val t0 = System.nanoTime()
            val got = planned.check(edit.toLong(), files, emptySet())
            val tInc = (System.nanoTime() - t0) / 1_000_000
            val t1 = System.nanoTime()
            val want = reference.check(edit.toLong(), files, rules)
            val tFull = (System.nanoTime() - t1) / 1_000_000
            val stale = compare(files.map { it.path }, want, got)
            staleVerdicts += stale.size
            val plan = assertNotNull(planned.lastPlan)
            if (edit > 0) {
                incrementalTotal += tInc; fullTotal += tFull
                if (plan.path == PlanPath.INCREMENTAL) engaged++ else fallbacks += "e$edit $label: ${plan.reason} ${plan.detail}"
            }
            log.append("e$edit %-12s %-11s inc=%5d ms full=%5d ms dirty=%3d waves=%d changed=%d new=%d del=%d unverdicted=%d stale=%d%n".format(
                label, plan.path, tInc, tFull, plan.dirtyFiles, plan.waves, plan.changedFiles, plan.newFiles, plan.deletedFiles,
                plan.unverdictedFiles, stale.size))
            assertTrue(stale.isEmpty(), "edit $edit ($label) left ${stale.size} stale verdicts: ${stale.take(5)}\n$log")
            edit++
        }

        step("cold")
        assertEquals(PlanPath.FULL, planned.lastPlan!!.path)
        assertEquals(FullReason.COLD, planned.lastPlan!!.reason)
        for (i in 1..EDITS) {
            val kind = CYCLE[(i - 1) % CYCLE.size]
            project.apply(kind)
            step(kind)
        }
        println(log)
        println("incremental total=${incrementalTotal} ms, fresh full total=${fullTotal} ms over $EDITS edits; engaged=$engaged fallbacks=$fallbacks")
        assertEquals(0, staleVerdicts)
        // The planner engages for (nearly) every edit; a full compile is allowed only to refresh the snapshot.
        assertTrue(engaged >= EDITS * 9 / 10, "the planner took the incremental path for $engaged of $EDITS edits: $fallbacks\n$log")
        assertTrue(fallbacks.all { "NEEDS_RESNAPSHOT" in it || "THRESHOLD" in it }, fallbacks.toString())
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
