package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.SdkLevels
import dev.jasonpearson.krit.fir.incremental.FullReason
import dev.jasonpearson.krit.fir.incremental.IncrementalStage
import dev.jasonpearson.krit.fir.incremental.IncrementalTestHook
import dev.jasonpearson.krit.fir.incremental.PlanPath
import dev.jasonpearson.krit.fir.incremental.PlanTelemetry
import dev.jasonpearson.krit.fir.incremental.SliceOutcome
import dev.jasonpearson.krit.fir.incremental.TieredDecision
import dev.jasonpearson.krit.fir.incremental.VerdictCache
import dev.jasonpearson.krit.fir.incremental.VerdictKeys
import dev.jasonpearson.krit.fir.incremental.canonicalPath
import dev.jasonpearson.krit.fir.incremental.sha1Hex
import dev.jasonpearson.krit.fir.isIsolatable
import java.io.File

/**
 * Serves a `check` request from the session's recorded state instead of a whole-module compile (#872). Behind
 * `KRIT_FIR_INCREMENTAL`; see [AnalysisSession.incrementalPlanning].
 *
 * The request's module is its source dirs plus the requested files. Against the state's content hashes it finds the
 * files that changed, are new or were deleted, plus requested files that have no valid verdict (never checked, or
 * checked under other rules, configs or per-file inputs). Then it runs waves of [AnalysisSession.sliceCheck]:
 *  - wave 1 compiles those files (the slice adds binary-stale files and closes sealed hierarchies itself);
 *  - after each wave, the files whose surface changed add their dependents for the next wave: the files that
 *    looked up a changed symbol (also through a subclass), the files declaring the same top-level name, the files of
 *    the supertypes of a changed class and of the subclasses of a class whose members changed. A file that an
 *    earlier wave compiled is compiled again when a later wave changed a surface it uses, since the earlier compile
 *    read the old binary.
 * It stops when a wave adds nothing. The result is then assembled from the per-file verdict cache: fresh verdicts
 * for the compiled files, cached ones for the rest, in the shape of a full check.
 *
 * It gives up in favour of the full [AnalysisSession.checkCompilation] (which also rebuilds the snapshot) for: no
 * state yet, a slice that cannot be used ([AnalysisSession.sliceFallback]), too many stale files kept in every
 * slice ([AnalysisSession.needsResnapshot]), a [TieredRulePolicy][dev.jasonpearson.krit.fir.incremental.TieredRulePolicy]
 * that asks for it, a dirty set above [PlannerLimits.threshold][dev.jasonpearson.krit.fir.incremental.PlannerLimits],
 * more waves than [PlannerLimits.maxWaves][dev.jasonpearson.krit.fir.incremental.PlannerLimits], a location-less
 * compiler error, a package that lost its last declaring file, lost state, and any exception.
 *
 * What makes a cached verdict reusable is that nothing it depends on changed: its file's content hash and verdict
 * key match (necessary, not sufficient), and no wave's dirty set reached it (lookup dependents, subclass scopes,
 * redeclarations, supertypes and subclasses of every surface that changed). A requested file that has no matching
 * entry is compiled in wave 1, or the request falls back. Dependents the lookups cannot see (a star import of a
 * vanished package) are the fallbacks' job.
 *
 * Known gap: a rule error without a file (a checker that threw outside any file) is reported on every compiled file
 * of a full check, but only on the files of a slice.
 */
internal class IncrementalPlanner(private val session: AnalysisSession) {

    private class Request(
        val id: Long, val files: List<FileRef>, val ruleConfigs: Map<String, Map<String, Any?>>,
        val testFiles: Set<String>, val scanPaths: Map<String, String>, val sdkLevels: Map<String, SdkLevels>,
    )

    private class Counters {
        var requested = 0
        var changed = 0
        var new = 0
        var deleted = 0
        var unverdicted = 0
        var waves = 0
        var addedByStaleness = 0
        var addedBySealedClosure = 0
        val waveMillis = ArrayList<Long>()
        val compiled = LinkedHashSet<String>()

        /** A wave committed to the state: giving up now must replace or drop that state. */
        var partial = false
    }

    /** Control flow: the request is served by the full compile instead. */
    private class GiveUp(val reason: FullReason, val detail: String? = null) : RuntimeException(detail, null, false, false)

    fun serve(
        id: Long, files: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>>, testFiles: Set<String>,
        scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
    ): BatchResult = synchronized(session.snapshotLock) {
        val start = System.nanoTime()
        val request = Request(id, files, ruleConfigs, testFiles, scanPaths, sdkLevels)
        val counters = Counters()

        val enabledIds = FirRuleDiscovery.enabled(FirRuleCompileContext(enabledRules)).mapTo(LinkedHashSet()) { it.ruleId }
        val tiered = FirRuleDiscovery.wholeCompilationRuleIds().toSet()
        val decision = try {
            session.tieredRulePolicy.decide(enabledIds, tiered)
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            TieredDecision.UseFullCompile("the tiered-rule policy failed: ${e.javaClass.simpleName}: ${e.message}")
        }
        when (decision) {
            is TieredDecision.UseFullCompile ->
                // The planner diffs against the state's own recorded hashes, so a request that bypasses the state
                // (no recording, no snapshot, no commit) leaves it correct, and costs what a plain check costs.
                return finish(
                    session.checkCompilation(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, record = false),
                    PlanPath.FULL, FullReason.TIERED_RULES, decision.reason, counters, start)
            is TieredDecision.RunIncrementally -> {
                // "All rules" stays the empty set a request spells it with: plugin rules are not in the discovery.
                val rules = if (decision.rules == enabledIds) enabledRules else decision.rules
                val commitsBefore = session.fullCommits
                val gaveUp = try {
                    val result = attempt(request, rules, counters)
                    return finish(result, PlanPath.INCREMENTAL, null, null, counters, start)
                } catch (g: GiveUp) {
                    g
                } catch (e: Throwable) {
                    if (!isIsolatable(e)) {
                        // Cancelled or interrupted between waves: the waves already merged are a half-applied edit.
                        if (counters.partial && session.fullCommits == commitsBefore) session.resetIncremental()
                        throw e
                    }
                    System.err.println("krit-fir: incremental planning failed (${e.javaClass.name}: ${e.message}); using the full compile")
                    GiveUp(FullReason.EXCEPTION, "${e.javaClass.simpleName}: ${e.message}")
                }
                return finish(runFull(request, rules, counters), PlanPath.FULL, gaveUp.reason, gaveUp.detail, counters, start)
            }
        }
    }

    // The whole-module check, which replaces the state and rebuilds the snapshot. If waves already changed the state
    // and the full check did not replace it (it crashed or recorded nothing), the half-applied state is dropped.
    private fun runFull(r: Request, rules: Set<String>, c: Counters): BatchResult {
        val before = session.fullCommits
        try {
            return session.checkCompilation(r.id, r.files, rules, r.ruleConfigs, r.testFiles, r.scanPaths, r.sdkLevels)
        } finally {
            if (c.partial && session.fullCommits == before) session.resetIncremental()
        }
    }

    private fun finish(
        result: BatchResult, path: PlanPath, reason: FullReason?, detail: String?, c: Counters, start: Long,
    ): BatchResult {
        val telemetry = PlanTelemetry(
            path, reason, detail, c.requested, c.changed, c.new, c.deleted, c.unverdicted, c.compiled.size, c.waves,
            c.addedByStaleness, c.addedBySealedClosure, c.waveMillis.toList(), (System.nanoTime() - start) / 1_000_000,
        )
        session.lastPlan = telemetry
        if (DEBUG) System.err.println(telemetry)
        return result
    }

    private fun attempt(r: Request, rules: Set<String>, c: Counters): BatchResult {
        val state = session.incrementalState
        if (state.compileKey == null) throw GiveUp(FullReason.COLD)
        if (session.verdicts.generation != state.generation) {
            throw GiveUp(FullReason.STATE_LOST, "verdicts at generation ${session.verdicts.generation}, state at ${state.generation}")
        }
        session.sliceFallback()?.let { throw GiveUp(FullReason.SLICE_UNAVAILABLE, it.name) }
        // A module with a compile error cannot be snapshotted (the backend never runs), so a full check would only
        // pay for nothing: carry the stale files until the module is clean again.
        if (session.needsResnapshot(session.plannerLimits.staleFraction, session.plannerLimits.staleMinFiles) &&
            session.verdicts.firstError(emptyList()) == null
        ) throw GiveUp(FullReason.NEEDS_RESNAPSHOT, "binary-stale and removed files passed the bound")

        IncrementalTestHook.onStage?.invoke(IncrementalStage.PLAN)
        val shape = session.shapeOf(r.id, r.files, rules, r.ruleConfigs, r.testFiles, r.scanPaths, r.sdkLevels)
        val requested = LinkedHashMap<String, FileRef>()
        for (ref in shape.compiled) requested.putIfAbsent(canonicalPath(ref.path), ref)
        c.requested = requested.size
        for ((file, ref) in requested) {
            if (!File(file).isFile) throw GiveUp(FullReason.MISSING_VERDICT, "requested file is not on disk: ${ref.path}")
        }
        val keys = session.verdictKeysFor(rules, r.ruleConfigs, r.testFiles, r.scanPaths, r.sdkLevels)
        val verdicts = session.verdicts

        // What changed since the state: against its content hashes, over the whole module.
        val module = session.moduleFilesFor(shape.compiled.map { it.path })
        val hashes = state.contentHashes()
        val newFiles = module.filterTo(LinkedHashSet()) { it !in hashes }
        val deleted = hashes.keys.filterTo(LinkedHashSet()) { it !in module }
        val known = module.filter { it in hashes }
        val changedList = if (known.size > PARALLEL_HASH_FROM) known.parallelStream().filter { changedSince(it, hashes, requested[it], verdicts) }.toList()
        else known.filter { changedSince(it, hashes, requested[it], verdicts) }
        val changed = changedList.toCollection(LinkedHashSet())
        // Requested files with no valid verdict are compiled too, even when their content did not change.
        val unverdicted = requested.keys.filterTo(LinkedHashSet()) {
            it !in changed && it in hashes && verdicts.valid(it, hashes[it], keys.of(it)) == null
        }
        c.changed = changed.size; c.new = newFiles.size; c.deleted = deleted.size; c.unverdicted = unverdicted.size
        refreshSuppliedHashes(requested, changed, verdicts)
        // Packages a deleted or edited file declared: if none is left afterwards, star importers are stale.
        val packagesAtRisk = (deleted + changed).mapNotNullTo(LinkedHashSet()) { state.surfaceOf(it)?.pkg }
        val carried = state.binaryStale()

        // Verdicts of SerialVersionUIDInSerializableClass depend on the module-wide set of names reaching Serializable.
        val serializableEnabled = FirRuleDiscovery.enabled(FirRuleCompileContext(rules)).any { it.ruleId == SERIALIZABLE_RULE_ID }
        val serializableBefore = if (serializableEnabled) state.serializableNames() else emptySet()

        val limits = session.plannerLimits
        val threshold = limits.threshold(module.size)
        val removedPending = LinkedHashSet(deleted)
        var todo: Set<String> = LinkedHashSet<String>().apply { addAll(changed); addAll(newFiles); addAll(unverdicted) }
        while (todo.isNotEmpty() || removedPending.isNotEmpty()) {
            if (c.waves >= limits.maxWaves) throw GiveUp(FullReason.WAVE_LIMIT, "more than ${limits.maxWaves} waves")
            // The binary-stale files every slice carries are not the edit's: only what the edit made dirty counts.
            val dirty = LinkedHashSet(c.compiled).apply { addAll(todo); removeAll(carried) }
            if (dirty.size > threshold) throw GiveUp(FullReason.THRESHOLD, "${dirty.size} dirty files, threshold $threshold of ${module.size}")

            val staleBefore = state.binaryStale()
            val t0 = System.nanoTime()
            val outcome = session.sliceCheck(
                r.id, todo.map { requested[it] ?: FileRef(it) }, rules, r.ruleConfigs, r.testFiles, r.scanPaths, r.sdkLevels,
                removedPending, requested,
            )
            c.waveMillis += (System.nanoTime() - t0) / 1_000_000
            val checked = when (outcome) {
                is SliceOutcome.Fallback -> throw GiveUp(FullReason.SLICE_UNAVAILABLE, "${outcome.reason}${outcome.cause?.let { ": ${it.message}" }.orEmpty()}")
                is SliceOutcome.Checked -> outcome
            }
            c.partial = true
            c.waves++
            if (state.compileKey == null || verdicts.generation != state.generation) throw GiveUp(FullReason.STATE_LOST, "the slice's facts were dropped")
            if (checked.result.ownedCompilerError != null) throw GiveUp(FullReason.GLOBAL_COMPILER_ERROR, checked.result.ownedCompilerError)
            c.compiled += checked.compiled
            for (added in checked.added) if (added in staleBefore) c.addedByStaleness++ else c.addedBySealedClosure++

            // The state's changes cover this wave only: consume them now.
            val touched = checked.compiled + removedPending
            val next = LinkedHashSet<String>()
            next += state.dependentsWithSubclassScopes(touched)
            next += state.structuralDependentsOf(touched)
            next.removeAll(checked.compiled)
            next.retainAll(module)
            removedPending.clear()
            todo = next
        }

        // Slices see the rest of the module's classes through recorded facts, so they are right while the set stands.
        // If the edit moved it, files outside the slices may hold a stale verdict: the full check decides.
        if (serializableEnabled && state.serializableNames() != serializableBefore) {
            throw GiveUp(FullReason.SERIALIZABLE_NAMES_CHANGED, "the set of class names reaching Serializable changed")
        }
        for (pkg in packagesAtRisk) {
            if (!state.packageDeclared(pkg)) throw GiveUp(FullReason.PACKAGE_REMOVED, "package '$pkg' has no declaring file left")
        }
        return assemble(r.id, shape, requested, keys, state.contentHashes())
            ?: throw GiveUp(FullReason.MISSING_VERDICT, "a requested file has no valid verdict after the waves")
    }

    // The content-hash decision for one module file the state knows. A request's supplied hash is trusted only
    // against the one the file's verdict was last computed under; otherwise the content is hashed and compared with
    // what the state recorded.
    private fun changedSince(file: String, hashes: Map<String, String>, ref: FileRef?, verdicts: VerdictCache): Boolean {
        val supplied = ref?.contentHash?.takeIf { it.isNotBlank() }
        if (supplied != null && verdicts.get(file)?.suppliedHash == supplied) return false
        val now = try { sha1Hex(File(file).readBytes()) } catch (_: Exception) { return true }
        return now != hashes[file]
    }

    // A file whose supplied hash changed scheme or value but whose content did not keeps its verdict under the new hash.
    private fun refreshSuppliedHashes(requested: Map<String, FileRef>, changed: Set<String>, verdicts: VerdictCache) {
        for ((file, ref) in requested) {
            val supplied = ref.contentHash.takeIf { it.isNotBlank() } ?: continue
            val entry = verdicts.get(file) ?: continue
            if (file in changed || entry.suppliedHash == supplied) continue
            verdicts.put(file, VerdictCache.Entry(entry.contentHash, supplied, entry.verdictKey, entry.verdict))
        }
    }

    // The result of a full check, from the per-file verdicts of the requested files; null if one has no valid verdict.
    private fun assemble(
        id: Long, shape: AnalysisSession.RequestShape, requested: Map<String, FileRef>,
        keys: VerdictKeys, hashes: Map<String, String>,
    ): BatchResult? {
        val verdicts = session.verdicts
        val findings = ArrayList<Finding>()
        val errorFiles = LinkedHashMap(shape.skeleton.errorFiles)
        val ruleErrors = LinkedHashMap<String, LinkedHashMap<String, String>>()
        for ((file, ref) in requested) {
            val entry = verdicts.valid(file, hashes[file], keys.of(file)) ?: return null
            val verdict = entry.verdict
            for (finding in verdict.findings) findings += if (finding.path == ref.path) finding else finding.copy(path = ref.path)
            verdict.error?.let { errorFiles[ref.path] = it }
            for ((rule, message) in verdict.ruleErrors) ruleErrors.getOrPut(rule) { LinkedHashMap() }[ref.path] = message
        }
        // A full check counts every compiled request entry, duplicates included.
        val succeeded = shape.compiled.count { canonicalPath(it.path).let { file -> verdicts.get(file)?.verdict?.error == null } }
        val first = shape.compiled.firstNotNullOfOrNull { verdicts.get(canonicalPath(it.path))?.verdict?.error }
            ?: verdicts.firstError(emptyList())
        return shape.skeleton.copy(
            id = id, succeeded = succeeded, findings = findings, errorFiles = errorFiles, ruleErrors = ruleErrors,
            firstCompilerError = first,
        )
    }

    private companion object {
        val DEBUG = System.getenv("KRIT_FIR_INCREMENTAL_DEBUG") == "1"
        const val PARALLEL_HASH_FROM = 256
        const val SERIALIZABLE_RULE_ID = "SerialVersionUIDInSerializableClass"
    }
}
