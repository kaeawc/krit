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
 * its callers, and as a declaration-only rename that breaks them followed by the repair). A Serializable chain with
 * nested classes and Throwable subclasses whose supertype names hit the set of names reaching Serializable gets
 * edits that move that set (add, rename, re-parent, delete; [SyntheticProject.SET_MOVING]) and one that does not.
 *
 * After every edit the planned check of all files (an incremental session under the default policy: no built-in
 * rule is tiered, so the incremental path engages with every rule on, SerialVersionUIDInSerializableClass included)
 * is compared, file by file, with a full check by a plain session that has no incremental state, run with the same
 * rules: findings, compiler-error gate and rule errors must be equal, so there is no stale verdict. The run also
 * asserts that the planner engaged, and reports how long each side took.
 *
 * The edits of the Serializable chain, and every fifth other edit, are first checked on only the edited files with a
 * rule set that leaves SerialVersionUIDInSerializableClass out, and then on all files with the rule on again: the
 * planner must not trust a verdict across a request that moved the recorded class-name facts while the rule was off.
 * An edit that moves the set must never be served incrementally (it falls back with SERIALIZABLE_NAMES_CHANGED).
 *
 * It compiles the module about 70 times, so it is tagged [TAG] and runs in its own Gradle task
 * (`incrementalPlannerTest`), not in `test`.
 */
@Tag(IncrementalPlannerDifferentialTest.TAG)
class IncrementalPlannerDifferentialTest {
    companion object {
        const val TAG = "incremental-planner"
        private val FILES = System.getProperty("krit.fir.planner.files")?.toInt() ?: 600
        private val EDITS = System.getProperty("krit.fir.planner.edits")?.toInt() ?: 56

        private val STALE_FRACTION = System.getProperty("krit.fir.planner.staleFraction")?.toDouble()
        private val STALE_MIN = System.getProperty("krit.fir.planner.staleMin")?.toInt()

        // The Serializable chain edits come first, while the module compiles: a full check of a module with a compile
        // error builds no snapshot, and the signature edits below leave it broken for stretches.
        private val CYCLE = listOf(
            "serAdd", "serBody", "serNested", "serRename", "serReparent", "body", "serErr", "serDelete",
            "serNested", "serReparent", "serRename", "serBody", "serAdd", "serErr", "serDelete", "serBody",
            "body", "sig", "add", "overload", "comment", "sealed", "delete", "rename", "renameBreak", "renameRepair",
            "shadow", "redeclare", "abstractBreak", "abstractRepair", "multi", "deleteBreak", "deleteRepair", "sig", "redeclare", "multi",
            "body", "add", "redeclare", "comment", "sealed", "multi", "abstractBreak", "abstractRepair", "overload", "shadow",
            "redeclare", "delete", "rename", "sig", "multi", "body", "redeclare", "sealed", "comment", "body",
        )
        private const val SERIAL = "SerialVersionUIDInSerializableClass"
    }

    @TempDir lateinit var tmp: Path

    @Test fun incrementalChecksEqualFreshFullChecksAcrossAMixedEditRun() {
        val src = tmp.resolve("src").toFile()
        val project = SyntheticProject(src, FILES).also { it.generate() }
        assertEquals(emptyList(), FirRuleDiscovery.wholeCompilationRuleIds(), "no rule is tiered, so the default policy engages")
        val rules = FirRuleDiscovery.rules.map { it.ruleId }.toSet()
        assertTrue(SERIAL in rules, "the planner differential runs with $SERIAL on")
        // The request that leaves the rule out, for the on/off rounds.
        val rulesWithoutSerial = rules - SERIAL

        val planned = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile() }
        val reference = AnalysisSession(project.sourceDirs, listOf(testStdlib), incrementalRecording = false, incrementalPlanning = false)

        if (STALE_FRACTION != null || STALE_MIN != null) {
            planned.plannerLimits = planned.plannerLimits.copy(
                staleFraction = STALE_FRACTION ?: planned.plannerLimits.staleFraction,
                staleMinFiles = STALE_MIN ?: planned.plannerLimits.staleMinFiles,
            )
        }

        var edit = 0
        val sampler = java.util.Random(5)
        // Every request names all files, or half of them chosen at random: with the files the edit touched left out
        // (a changed file that is not requested), or with them.
        fun requests(supplyHashes: Boolean, edited: List<String>): List<FileRef> {
            val all = project.files().sorted()
            val picked = when (edit % 4) {
                1 -> all.filter { it !in edited && sampler.nextBoolean() }
                3 -> all.filter { it in edited || sampler.nextBoolean() }
                else -> all
            }
            return picked.map { FileRef(it, if (supplyHashes) sha1Hex(File(it).readBytes()) else "") }
        }

        val log = StringBuilder()
        var incrementalTotal = 0L
        var fullTotal = 0L
        var engaged = 0
        var steps = 0
        var offSteps = 0
        var offEngaged = 0
        var eligible = 0
        var eligibleEngaged = 0
        var setMovingEdits = 0
        var setMovingFallbacks = 0
        val fallbacks = ArrayList<String>()
        val fallbackReasons = java.util.TreeMap<String, Int>()
        var staleVerdicts = 0
        val incrementalTimes = ArrayList<Long>()

        // [off]: the request names only the edited files and leaves SerialVersionUIDInSerializableClass out.
        // [setMoving]: the edit moves the set of names reaching Serializable; [mustFall]: this request is the first to
        // see it, so it must not be served incrementally. [excused]: a full check is expected
        // (the state was just rebuilt for another rule set, so most verdicts are missing).
        fun step(label: String, edited: List<String> = emptyList(), off: Boolean = false, setMoving: Boolean = false, excused: Boolean = false, mustFall: Boolean = setMoving) {
            // Alternate the request shape: with and without the content hashes Go supplies.
            val files = if (off) {
                edited.filter { File(it).isFile }.map { FileRef(it, if (edit % 2 == 0) sha1Hex(File(it).readBytes()) else "") }
            } else requests(supplyHashes = edit % 2 == 0, edited = edited)
            val plannedRules = if (off) rulesWithoutSerial else emptySet()
            val referenceRules = if (off) rulesWithoutSerial else rules
            val t0 = System.nanoTime()
            val got = planned.check(edit.toLong(), files, plannedRules)
            val tInc = (System.nanoTime() - t0) / 1_000_000
            val t1 = System.nanoTime()
            val want = reference.check(edit.toLong(), files, referenceRules)
            val tFull = (System.nanoTime() - t1) / 1_000_000
            val stale = compare(files.map { it.path }, want, got)
            staleVerdicts += stale.size
            val plan = assertNotNull(planned.lastPlan)
            if (edit > 0) {
                incrementalTotal += tInc; fullTotal += tFull; incrementalTimes += tInc
                steps++
                if (off) offSteps++
                val inc = plan.path == PlanPath.INCREMENTAL
                if (inc) engaged++ else fallbacks += "e$edit $label${if (off) " (rule off)" else ""}: ${plan.reason} ${plan.detail}"
                if (inc && off) offEngaged++
                if (!inc) fallbackReasons.merge(plan.reason.toString(), 1, Int::plus)
                if (!setMoving && !excused) { eligible++; if (inc) eligibleEngaged++ }
                if (mustFall) {
                    setMovingEdits++
                    assertTrue(!inc, "edit $edit ($label) moves the set of names reaching Serializable but was served incrementally\n$log")
                    if (plan.reason == FullReason.SERIALIZABLE_NAMES_CHANGED) setMovingFallbacks++
                }
            }
            log.append("e$edit %-12s %-3s %-11s inc=%5d ms full=%5d ms dirty=%3d waves=%d changed=%d new=%d del=%d unverdicted=%d reason=%s stale=%d%n".format(
                label, if (off) "off" else "on", plan.path, tInc, tFull, plan.dirtyFiles, plan.waves, plan.changedFiles, plan.newFiles,
                plan.deletedFiles, plan.unverdictedFiles, plan.reason, stale.size))
            assertTrue(stale.isEmpty(), "edit $edit ($label) left ${stale.size} stale verdicts: ${stale.take(5)}\n$log")
            edit++
        }

        step("cold")
        assertEquals(PlanPath.FULL, planned.lastPlan!!.path)
        assertEquals(FullReason.COLD, planned.lastPlan!!.reason)
        for (i in 1..EDITS) {
            val kind = CYCLE[(i - 1) % CYCLE.size]
            val change = project.apply(kind)
            val touched = change.changed + change.added + change.deleted
            val setMoving = kind in SyntheticProject.SET_MOVING
            if (setMoving || i % 5 == 0) {
                // Rule off on the edited files only, then everything with the rule on again.
                if (touched.any { File(it).isFile }) {
                    step(kind, touched, off = true, setMoving = setMoving)
                    step(kind, touched, setMoving = setMoving, excused = setMoving, mustFall = false)
                    continue
                }
            }
            step(kind, touched, setMoving = setMoving)
        }
        println(log)
        val incTimes = incrementalTimes.sorted()
        println("planned p50=${incTimes[incTimes.size / 2]} ms p95=${incTimes[(incTimes.size * 95 / 100).coerceAtMost(incTimes.size - 1)]} ms stale-bound=(${planned.plannerLimits.staleFraction}, ${planned.plannerLimits.staleMinFiles})")
        println("incremental total=${incrementalTotal} ms, fresh full total=${fullTotal} ms over $EDITS edits ($steps checks); engaged=$engaged " +
            "rule-off checks=$offSteps (engaged $offEngaged) set-moving edits=$setMovingEdits (SERIALIZABLE_NAMES_CHANGED $setMovingFallbacks) " +
            "fallback reasons=$fallbackReasons stale=$staleVerdicts fallbacks=$fallbacks")
        assertEquals(0, staleVerdicts)
        assertTrue(setMovingEdits > 0 && setMovingFallbacks > 0, "no edit exercised SERIALIZABLE_NAMES_CHANGED\n$log")
        // The planner engages for (nearly) every edit that does not move the set; a full compile is allowed only to
        // refresh the snapshot, or after the set moved.
        assertTrue(eligibleEngaged >= eligible * 9 / 10, "the planner took the incremental path for $eligibleEngaged of $eligible checks: $fallbacks\n$log")
        assertTrue(
            fallbacks.all { "NEEDS_RESNAPSHOT" in it || "THRESHOLD" in it || "PACKAGE_REMOVED" in it || "SERIALIZABLE_NAMES_CHANGED" in it },
            fallbacks.toString(),
        )
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
