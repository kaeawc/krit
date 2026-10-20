package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import java.io.File
import java.nio.file.Files
import kotlin.test.assertTrue

/**
 * The tiered-rules study (#740, tiered-rules-findings.md): replays an edit sequence on a corpus and, after every edit,
 * serves the module's verdicts with several strategies and compares each, file by file and split into tiered and
 * other rules, with a fresh full compile of the edited module.
 *
 * Strategies (all share one recording session; every slice is the production `sliceCheck`):
 *  - `plain`: the step-4 slice only (changed files + lookup dependents of changed surfaces, waves), every rule's
 *    verdict taken from it for the compiled files and from the cache for the rest: what running tiered rules on a
 *    slice gives.
 *  - `A<k>`: option A, the plain set plus T_k = files whose depth-k lookup closure contains a changed file, compiled
 *    together with T_k's depth-k closure in one slice; tiered verdicts for T_k files from it, cache for the rest.
 *  - `G<k>`: option B with the trigger gate: as A<k>, but only T_k files that hold a tiered rule's entry lookup
 *    (or reach a seed lookup) are expanded and refreshed (see [TieredTriggers]).
 *  - the full compile itself is the reference (and the "always full" strategy's cost).
 *
 * Modes (`-Pkrit.fir.tiered.mode=`): `synthetic` (the spike generator with tiered chains), `kritfir` (krit-fir's own
 * sources, real edits), `static` (closure and gate sizes only, one full compile: `-Pkrit.fir.tiered.roots=` a
 * comma-separated list of source roots), `depth` (runs [TieredDepthProbeTest] instead; this test skips).
 */
@Tag(TieredResearch.TAG)
class TieredResearch {
    companion object {
        const val TAG = "tiered-research"
        private const val INF = Int.MAX_VALUE
    }

    private val mode = System.getProperty("krit.fir.tiered.mode") ?: "synthetic"
    private val edits = System.getProperty("krit.fir.tiered.edits")?.toIntOrNull() ?: 30
    private val size = System.getProperty("krit.fir.tiered.size")?.toIntOrNull() ?: 600
    private val seed = System.getProperty("krit.fir.tiered.seed")?.toLongOrNull() ?: 11L
    private val depths = (System.getProperty("krit.fir.tiered.depths") ?: "1,2,3,inf").split(",")
        .map { if (it == "inf") INF else it.toInt() }
    private val ungated = System.getProperty("krit.fir.tiered.ungated")?.let { it != "false" } ?: true

    private fun depthName(k: Int) = if (k == INF) "inf" else k.toString()

    @Test
    fun run() {
        val work = Files.createTempDirectory("krit-fir-tiered").toFile()
        try {
            when (mode) {
                "synthetic" -> {
                    val gen = TieredGen(File(work, "proj"), size, seed).also { it.generate() }
                    replay(gen, listOf(testStdlib), work, kotlinArgs = emptyList())
                }
                "kritfir" -> {
                    val roots = listOf("src/main/kotlin", "../krit-rule-api/src/main/kotlin", "../krit-rule-api/build/generated/source/ruleApiVersion/main")
                    for (r in roots) check(File(r).isDirectory) { "missing $r (build krit-rule-api first)" }
                    val proj = TieredRealProj(roots, kritFirClasspath(), File(work, "proj"), seed)
                    replay(proj, proj.classpath, work, kotlinArgs = listOf("-Xcontext-parameters"))
                }
                "static" -> staticSizes(work)
                "depth" -> println("depth mode: see TieredDepthProbeTest")
                else -> error("unknown mode $mode")
            }
        } finally {
            work.deleteRecursively()
        }
    }

    private fun kritFirClasspath(): List<String> =
        System.getProperty("java.class.path").split(File.pathSeparator).filter {
            val n = File(it).name
            (n.startsWith("kotlin-compiler-") && !n.contains("embeddable")) || n.startsWith("kotlin-stdlib-2") || n.startsWith("annotations-")
        }

    private class Stats {
        val staleTiered = ArrayList<Int>()
        val staleOther = ArrayList<Int>()
        val compiled = ArrayList<Int>()
        val ms = ArrayList<Long>()
        val extra = ArrayList<Int>()
        val refreshed = ArrayList<Int>()
        var staleEdits = 0
    }

    private fun session(proj: TieredProj, cp: List<String>, kotlinArgs: List<String>, record: Boolean, snapshots: File? = null) =
        AnalysisSession(proj.sourceDirs, cp, "", kotlinArgs, incrementalRecording = record).also {
            if (snapshots != null) it.snapshotRootOverride = snapshots
        }

    private fun allFiles(proj: TieredProj) = proj.texts.keys.map(::canon).sorted()

    private fun replay(proj: TieredProj, cp: List<String>, work: File, kotlinArgs: List<String>) {
        println("## corpus: ${proj.describe()}; edits=$edits seed=$seed depths=${depths.map(::depthName)}")
        val rec = session(proj, cp, kotlinArgs, record = true, snapshots = File(work, "snapshots"))
        val t0 = System.nanoTime()
        val first = rec.check(1, allFiles(proj).map { FileRef(it) }, emptySet())
        println("## cold full check + snapshot: ${(System.nanoTime() - t0) / 1_000_000} ms, errorFiles=${first.errorFiles.size}, snapshot=${rec.incrementalState.snapshot != null}")
        check(rec.incrementalState.snapshot != null) { "no snapshot (${rec.incrementalState.fallbackReason()}): ${first.errorFiles.values.take(3)}" }

        val gen = proj as? TieredGen
        val strategies = buildList {
            add("plain")
            if (gen != null) add("R")
            if (ungated) for (k in depths) add("A${depthName(k)}")
            for (k in depths) add("G${depthName(k)}")
        }
        val caches = strategies.associateWith { HashMap<String, SplitVerdict>() }
        for (f in allFiles(proj)) for (c in caches.values) c[f] = first.splitVerdictOf(f)
        val stats = strategies.associateWith { Stats() }
        val refMs = ArrayList<Long>()
        val tieredTruthFiles = HashSet<String>()
        var gateMisses = 0
        val byKind = HashMap<String, MutableList<String>>()

        for (n in 1..edits) {
            val e = proj.nextEdit(null)
            val changed = (e.changed + e.added).map(::canon).toSet()
            val deleted = e.deleted.map(::canon).toSet()
            val files = allFiles(proj)

            // Reference: a fresh frontend-only full compile of the edited module.
            val tr = System.nanoTime()
            val plainSession = session(proj, cp, kotlinArgs, record = false)
            val ref = plainSession.check(1, files.map { FileRef(it) }, emptySet())
            plainSession.dispose()
            refMs += (System.nanoTime() - tr) / 1_000_000
            val truth = files.associateWith { ref.splitVerdictOf(it) }
            for ((f, v) in truth) if (v.tiered.isNotEmpty()) tieredTruthFiles += f

            // Plain step-4 slice: changed files, then lookup dependents of changed surfaces, in waves.
            val state = rec.incrementalState
            val plainResults = HashMap<String, SplitVerdict>()
            val p = LinkedHashSet<String>()
            var wave: Set<String> = changed
            var removed: Set<String> = deleted
            var plainMs = 0L
            var fellBack = false
            for (w in 0 until 8) {
                if (wave.isEmpty() && removed.isEmpty()) break
                val tw = System.nanoTime()
                val out = rec.sliceCheck(1, wave.map { FileRef(it) }, emptySet(), removedFiles = removed)
                plainMs += (System.nanoTime() - tw) / 1_000_000
                if (out !is SliceOutcome.Checked) { fellBack = true; println("   fallback: ${(out as SliceOutcome.Fallback).reason} ${out.cause}"); break }
                p += out.compiled
                for (f in out.compiled) plainResults[f] = out.result.splitVerdictOf(f)
                val surf = out.compiled.filter { state.surfaceChanged(it) } + removed
                val next = HashSet(state.dependentsOf(surf))
                for (f in surf) {
                    val ch = state.changeOf(f) ?: continue
                    for (scope in ch.scopes) next += state.subclassFilesOf(scope)
                    for (sym in ch.symbols) next += state.filesDeclaring(sym)
                }
                next -= p
                wave = next.filter { File(it).isFile }.toSet()
                removed = emptySet()
            }
            if (fellBack) {
                // Rebuild like the planner would: a full recording check, every strategy refreshed from it.
                val full = rec.check(1, files.map { FileRef(it) }, emptySet())
                for (c in caches.values) { c.clear(); for (f in files) c[f] = full.splitVerdictOf(f) }
                continue
            }

            // Closures over the state after the plain slice (the compiled files carry their new lookups).
            val cl = LookupClosure(state)
            val sets = LinkedHashMap<String, Pair<Set<String>, Set<String>>>() // strategy -> (compile set, refreshed set)
            sets["plain"] = p to p
            if (gen != null) {
                // Exact read tracking (the generator knows each app file's Keys chain): refresh the files whose read
                // set holds a changed file, with their read sets in source.
                val read = files.associateWith { f -> gen.readSet(f).map(::canon).toSet() }
                val t = HashSet(p); t += changed
                for ((f, rs) in read) if (rs.any { it in changed }) t += f
                t.retainAll(files.toSet())
                sets["R"] = (p + t + t.flatMap { read[it].orEmpty() }).toSet() to t
            }
            for (k in depths) {
                val t = HashSet(p); t += changed; t += cl.affected(changed, k)
                t.retainAll(files.toSet())
                if (ungated) sets["A${depthName(k)}"] = (p + t + cl.closure(t, k)).toSet() to t
                val tg = t.filter { TieredTriggers.gated(state, cl, it, k) }.toSet()
                sets["G${depthName(k)}"] = (p + tg + cl.closure(tg, k)).toSet() to (p + tg)
            }

            // Compile each distinct set once.
            val compiledResults = HashMap<Set<String>, Pair<BatchResult?, Long>>()
            compiledResults[p] = null to plainMs
            for ((_, pair) in sets) {
                val set = pair.first
                if (set in compiledResults) continue
                val ts = System.nanoTime()
                val out = rec.sliceCheck(1, set.map { FileRef(it) }, emptySet())
                val ms = (System.nanoTime() - ts) / 1_000_000
                compiledResults[set] = ((out as? SliceOutcome.Checked)?.result ?: error("expanded slice fell back: $out")) to ms
            }

            val line = StringBuilder("edit $n ${e.kind} |P|=${p.size} ref=${refMs.last()}ms plain=${plainMs}ms")
            for (s in strategies) {
                val (set, refreshed) = sets.getValue(s)
                val (result, ms) = compiledResults.getValue(set)
                val cache = caches.getValue(s)
                for (f in deleted) cache.remove(f)
                for (f in set) {
                    if (!File(f).isFile) { cache.remove(f); continue }
                    val fresh = if (result == null) plainResults[f] ?: continue else result.splitVerdictOf(f)
                    val old = cache[f]
                    cache[f] = if (s == "plain" || f in refreshed || old == null) fresh else fresh.copy(tiered = old.tiered)
                }
                var st = 0; var so = 0
                for (f in files) {
                    val want = truth.getValue(f)
                    val got = cache[f]
                    if (got == null) { st++; so++; continue }
                    if (got.tiered != want.tiered) st++
                    if (got.other != want.other || got.error != want.error || got.ruleErrors != want.ruleErrors) so++
                }
                val ss = stats.getValue(s)
                ss.staleTiered += st; ss.staleOther += so; ss.compiled += set.size; ss.ms += ms
                ss.extra += (set - p).size; ss.refreshed += refreshed.size
                if (st > 0 || so > 0) ss.staleEdits++
                line.append(" | $s n=${set.size} ${ms}ms stale=$st/$so")
                byKind.getOrPut("$s/${e.kind}") { ArrayList() } += "${set.size}:${ms}"
            }
            // Gate soundness: every file with a tiered finding now must pass the gate (at depth inf).
            for ((f, v) in truth) if (v.tiered.isNotEmpty() && !TieredTriggers.gated(state, cl, f, INF)) gateMisses++
            println(line)
        }

        println("## summary: ${proj.describe()}, $edits edits, reference full compile p50 ${pct(refMs, 0.5)} / p95 ${pct(refMs, 0.95)} ms")
        println("## files with a tiered finding at some point: ${tieredTruthFiles.size}; gate misses (file-edits with a tiered finding the gate would skip): $gateMisses")
        println("| strategy | edits with any stale verdict | stale tiered file-verdicts (sum) | stale other (sum) | files compiled p50 / p95 / max | extra over plain p50 / p95 / max | refreshed (tiered) p50 | ms p50 / p95 |")
        println("|---|---|---|---|---|---|---|---|")
        for (s in strategies) {
            val ss = stats.getValue(s)
            println(
                "| $s | ${ss.staleEdits}/$edits | ${ss.staleTiered.sum()} (max ${ss.staleTiered.maxOrNull()}) | ${ss.staleOther.sum()} | ${pctI(ss.compiled, 0.5)} / ${pctI(ss.compiled, 0.95)} / ${ss.compiled.maxOrNull()} | " +
                    "${pctI(ss.extra, 0.5)} / ${pctI(ss.extra, 0.95)} / ${ss.extra.maxOrNull()} | ${pctI(ss.refreshed, 0.5)} | ${pct(ss.ms, 0.5)} / ${pct(ss.ms, 0.95)} |",
            )
        }
        println("## per edit kind (files compiled p50 : ms p50)")
        for ((key, xs) in byKind.toSortedMap()) {
            val nfiles = xs.map { it.substringBefore(':').toInt() }
            val ms = xs.map { it.substringAfter(':').toLong() }
            println("   $key n=${xs.size} files p50=${pctI(nfiles, 0.5)} max=${nfiles.maxOrNull()} ms p50=${pct(ms, 0.5)}")
        }
        rec.dispose()
        assertTrue(stats.getValue("plain").compiled.isNotEmpty())
    }

    /** One full recording compile of [roots]; per-file closure and gate sizes. No slices: the corpus may have errors. */
    private fun staticSizes(work: File) {
        val roots = (System.getProperty("krit.fir.tiered.roots") ?: error("set -Pkrit.fir.tiered.roots")).split(",")
        val stubs = System.getProperty("krit.fir.tiered.stubs") != "false"
        val cp = if (stubs) listOf(CompilerTestStubs.compile(work).path) else kritFirClasspath()
        val session = if (stubs) {
            CompilerTestStubs.session(roots, cp, record = true, snapshots = File(work, "snapshots"))
        } else {
            AnalysisSession(roots, cp, "", listOf("-Xcontext-parameters"), incrementalRecording = true).also { it.snapshotRootOverride = File(work, "snapshots") }
        }
        val files = roots.flatMap { r -> File(r).walkTopDown().onEnter { it.name !in TieredRealProj.prunedDirs }.filter { it.isFile && it.extension == "kt" }.map { canon(it.path) }.toList() }
        val t0 = System.nanoTime()
        val result = session.check(1, files.map { FileRef(it) }, emptySet())
        println("## static: ${files.size} files, full check ${(System.nanoTime() - t0) / 1_000_000} ms, errorFiles=${result.errorFiles.size}, tiered findings=${result.findings.count { it.rule in tieredRuleIds }}")
        for (f in result.findings.filter { it.rule in tieredRuleIds }.sortedBy { it.rule }) {
            println("   tiered finding: ${f.rule} ${f.path.substringAfterLast('/')}:${f.line} ${f.message.take(100)} error=${f.path in result.errorFiles}")
        }
        val state = session.incrementalState
        val cl = LookupClosure(state)
        val known = files.filter { it in cl.files }
        println("## files recorded: ${known.size}")
        val sink = known.filter { TieredTriggers.sinkRules(state, it).isNotEmpty() }
        val seed = known.filter { TieredTriggers.hasSeed(state, it).isNotEmpty() }
        val sinkByRule = HashMap<String, Int>()
        for (f in known) for (r in TieredTriggers.sinkRules(state, f) + TieredTriggers.hasSeed(state, f)) sinkByRule.merge(r, 1, Int::plus)
        println("## files with a sink lookup: ${sink.size}, with a seed lookup: ${seed.size}; by rule: ${sinkByRule.toSortedMap()}")
        val gatedMiss = result.findings.filter { it.rule in tieredRuleIds && it.rule in TieredTriggers.gatedRules }
            .map { canon(it.path) }.distinct().filter { !TieredTriggers.gated(state, cl, it, INF) }
        println("## files with a gated-rule tiered finding the gate would skip: ${gatedMiss.size} ${gatedMiss.take(5)}")
        println("| depth k | closure size per file p50 / p95 / max | affected by a body edit of one file (reverse closure) p50 / p95 / max | gated files whose closure must be in source | union of gated files' closures |")
        println("|---|---|---|---|---|")
        for (k in depths) {
            val sizes = known.map { cl.closure(listOf(it), k).size }
            // Reverse closure: for each file G, how many files' depth-k closure contains G.
            val reverse = HashMap<String, Int>()
            for (f in known) for (g in cl.closure(listOf(f), k)) reverse.merge(g, 1, Int::plus)
            val rev = known.map { reverse[it] ?: 0 }
            val gated = known.filter { TieredTriggers.gated(state, cl, it, k) }
            val union = (gated + cl.closure(gated, k)).toSet()
            println("| ${depthName(k)} | ${pctI(sizes, 0.5)} / ${pctI(sizes, 0.95)} / ${sizes.maxOrNull()} | ${pctI(rev, 0.5)} / ${pctI(rev, 0.95)} / ${rev.maxOrNull()} | ${gated.size} | ${union.size} |")
        }
        // Per-edit estimate: a body edit of file G must refresh the gated files whose closure contains G, with their closures.
        for (k in depths) {
            val perFile = known.map { g ->
                val aff = cl.affected(listOf(g), k).filter { TieredTriggers.gated(state, cl, it, k) } + listOf(g).filter { TieredTriggers.gated(state, cl, it, k) }
                (aff + cl.closure(aff, k)).toSet().size
            }
            val zero = perFile.count { it == 0 }
            println("## gated expanded set for a body edit of each file, depth ${depthName(k)}: p50 ${pctI(perFile, 0.5)} / p95 ${pctI(perFile, 0.95)} / max ${perFile.maxOrNull()}; zero for $zero of ${known.size} files")
        }
        session.dispose()
    }
}
