package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.AnalyzeOutcome
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * #740: the oracle answers of a planned `analyzeWithCheck` equal a fresh whole-module oracle compile across the
 * planner differential's mixed edit run ([SyntheticProject]): per-file facts, the `dependencies` table and the
 * `cacheDeps` edges, after every edit. Edits the planner serves from slices must also be answered from the cached
 * facts (no whole-module oracle compile), except right after a full check refilled them.
 *
 * Tagged like [IncrementalPlannerDifferentialTest]: it runs in the `incrementalPlannerTest` Gradle task.
 */
@Tag(IncrementalPlannerDifferentialTest.TAG)
class OracleIncrementalDifferentialTest {
    companion object {
        private val FILES = System.getProperty("krit.fir.planner.files")?.toInt()?.coerceAtMost(300) ?: 300
        // As in the planner differential, the Serializable chain edits come first, while the module compiles: a
        // module left broken by a later edit builds no snapshot, so the slices stop.
        private val CYCLE = listOf(
            "serAdd", "serBody", "serNested", "serRename", "serReparent", "body", "serErr", "serDelete",
            "body", "sig", "add", "overload", "comment", "sealed", "delete", "rename", "renameBreak", "renameRepair",
            "shadow", "redeclare", "abstractBreak", "abstractRepair", "multi", "deleteBreak", "deleteRepair", "sig", "multi",
        )
    }

    @TempDir lateinit var tmp: Path

    @Test fun plannedOracleAnswersEqualFreshCompilesAcrossAMixedEditRun() {
        val src = tmp.resolve("src").toFile()
        val project = SyntheticProject(src, FILES).also { it.generate() }
        val rules = FirRuleDiscovery.rules.map { it.ruleId }
        val planned = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile() }
        val reference = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = false, incrementalPlanning = false)

        val log = StringBuilder()
        var incrementalSteps = 0
        var fromCache = 0
        var mismatches = 0
        var edit = 0

        fun step(label: String) {
            val files = project.files().sorted()
            val request = CheckRequest(id = edit.toLong(), command = "check", files = files.map { FileRef(it) }, rules = rules)
            val t0 = System.nanoTime()
            val (got, checked) = planned.analyzeWithCheck(files, request)
            val tPlanned = (System.nanoTime() - t0) / 1_000_000
            val t1 = System.nanoTime()
            val want = reference.analyzeFull(files)
            val tFresh = (System.nanoTime() - t1) / 1_000_000
            val plan = planned.lastPlan
            val diff = differences(want, got)
            if (diff.isNotEmpty()) mismatches++
            val incremental = checked?.plan?.path == "incremental"
            if (edit > 0 && incremental) {
                incrementalSteps++
                if (planned.lastOracleFromCache) fromCache++
            }
            log.append("e$edit %-14s %-11s fromCache=%-5s planned=%5d ms fresh=%5d ms reason=%s diff=%d%n".format(
                label, plan?.path, planned.lastOracleFromCache, tPlanned, tFresh, plan?.reason, diff.size))
            assertTrue(diff.isEmpty(), "edit $edit ($label): ${diff.take(5)}\n$log")
            edit++
        }

        step("cold")
        for (kind in CYCLE) {
            project.apply(kind)
            step(kind)
        }
        println(log)
        println("oracle differential: $incrementalSteps incremental checks, $fromCache answered from cached facts, $mismatches mismatches")
        assertEquals(0, mismatches)
        assertTrue(incrementalSteps > CYCLE.size / 2, "the planner engaged on only $incrementalSteps of ${CYCLE.size} edits\n$log")
        // A served-from-slices request uses the cached facts unless they were a generation behind.
        assertTrue(fromCache >= incrementalSteps * 9 / 10, "only $fromCache of $incrementalSteps incremental checks used the cached facts\n$log")
        planned.dispose()
        reference.dispose()
    }

    private fun differences(want: AnalyzeOutcome, got: AnalyzeOutcome): List<String> {
        val out = ArrayList<String>()
        val files = want.result.files.keys + got.result.files.keys
        for (f in files.sorted()) {
            if (want.result.files[f] != got.result.files[f]) out += "$f facts differ: want=${want.result.files[f]} got=${got.result.files[f]}"
        }
        if (want.result.dependencies.toSortedMap() != got.result.dependencies.toSortedMap()) {
            val keys = (want.result.dependencies.keys + got.result.dependencies.keys).filter { want.result.dependencies[it] != got.result.dependencies[it] }
            out += "dependencies differ at ${keys.take(5)}"
        }
        val wantDeps = want.cacheDeps.depPathsByFile.mapValues { it.value.toSortedSet() }.toSortedMap()
        val gotDeps = got.cacheDeps.depPathsByFile.mapValues { it.value.toSortedSet() }.toSortedMap()
        if (wantDeps != gotDeps) {
            val keys = (wantDeps.keys + gotDeps.keys).filter { wantDeps[it] != gotDeps[it] }
            out += "depPaths differ at ${keys.take(3).map { "$it want=${wantDeps[it]} got=${gotDeps[it]}" }}"
        }
        val wantPer = want.cacheDeps.perFileDeps.mapValues { it.value.toSortedMap() }.toSortedMap()
        val gotPer = got.cacheDeps.perFileDeps.mapValues { it.value.toSortedMap() }.toSortedMap()
        if (wantPer != gotPer) out += "perFileDeps differ at ${(wantPer.keys + gotPer.keys).filter { wantPer[it] != gotPer[it] }.take(3)}"
        return out
    }
}
