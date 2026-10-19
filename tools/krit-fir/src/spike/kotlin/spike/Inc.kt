package spike

import dev.jasonpearson.krit.fir.oracle.OracleCollector
import dev.jasonpearson.krit.fir.oracle.OracleCollectorRegistry
import java.io.File

/**
 * Steady-state incremental FIR check harness (FIR-only: every compile is the K2 CLI frontend pipeline).
 *
 * Strategies (--strategy):
 *  - cli:       shipped FrontendOnlyJvmPipeline over the whole module per edit (baseline)
 *  - warm:      F1, whole module per edit through WarmFrontendPhase (env + library sessions reused)
 *  - slice:     F2 through the *stock* frontend phase: sources = dirty files, rest of the module from the binary
 *               snapshot as same-module IC binaries (Services: IncrementalCompilationComponentsWithCustomScope)
 *  - warmslice: F1 + F2 (WarmFrontendPhase with the IC symbol provider)
 * Snapshot freshness (--snap): patch = (a) run the backend on each slice and patch the snapshot;
 *                              keep  = (b) keep every file whose binary is stale in the source set.
 * Dependents: compiler LookupTracker reverse index, gated by a FIR declaration-surface diff, in waves (fixpoint).
 */
class Inc(val o: Map<String, String>) {
    val n = (o["files"] ?: "2000").toInt()
    val edits = (o["edits"] ?: "60").toInt()
    val warmup = (o["warmup"] ?: "10").toInt()
    val seed = (o["seed"] ?: "7").toLong()
    val strategy = o["strategy"] ?: "slice"
    val snapMode = o["snap"] ?: "patch"
    val oracleEvery = (o["oracle"] ?: "0").toInt()
    val oracleSample = (o["oracleSample"] ?: "0").toInt()
    val threshold = (o["threshold"] ?: "400").toInt()
    val maxWaves = (o["maxWaves"] ?: "8").toInt()
    val track = o["track"] != "false"
    val depTracker = o["deptracker"] == "true"
    val kinds = o["kinds"]?.split(",")
    val kotlinArgs = o["kotlinArgs"]?.split(",")?.filter { it.isNotBlank() } ?: emptyList()
    val gcEvery = (o["gcEvery"] ?: "10").toInt()
    val dir = File(SCRATCH, o["dir"] ?: "inc-$strategy-$snapMode-$n")
    val logFile = o["out"]?.let { File(it) }

    private val isWarm = strategy == "warm" || strategy == "warmslice"
    private val isSlice = strategy == "slice" || strategy == "warmslice"

    lateinit var g: Proj
    val cache = HashMap<String, FileVerdict>()
    val surfaces = HashMap<String, FileSurface>()
    private val declIndex = HashMap<Sym, MutableSet<String>>() // declared symbol (class mark stripped) -> files
    private val classFile = HashMap<String, String>() // class fq -> declaring file

    private fun plainSym(k: Sym) = if (k.name.startsWith(FileSurface.CLASS_MARK)) Sym(k.scope, k.name.removePrefix(FileSurface.CLASS_MARK)) else k
    private fun classFq(k: Sym) = (if (k.scope.isEmpty()) "" else k.scope + ".") + k.name.removePrefix(FileSurface.CLASS_MARK)

    private fun setSurface(f: String, sf: FileSurface?) {
        surfaces[f]?.let { old -> for (k in old.decls.keys) { declIndex[plainSym(k)]?.remove(f); if (k.name.startsWith(FileSurface.CLASS_MARK)) classFile.remove(classFq(k), f) } }
        if (sf == null) { surfaces.remove(f); return }
        surfaces[f] = sf
        for (k in sf.decls.keys) { declIndex.getOrPut(plainSym(k)) { HashSet() }.add(f); if (k.name.startsWith(FileSurface.CLASS_MARK)) classFile[classFq(k)] = f }
    }

    /**
     * Files a surface change reaches that lookups cannot: the declaring files of the supertypes of an added, removed
     * or changed class (a sealed parent's binary lists its inheritors), and other files declaring an added or removed
     * top-level name (redeclaration / conflicting overloads need both sides in source).
     */
    private fun structural(f: String, old: FileSurface?, new: FileSurface?): Set<String> {
        val out = HashSet<String>()
        val o = old?.decls.orEmpty(); val n = new?.decls.orEmpty()
        for (k in o.keys + n.keys) {
            if (o[k] == n[k]) continue
            if (k.name.startsWith(FileSurface.CLASS_MARK)) {
                val fq = classFq(k)
                for (sup in old?.supers?.get(fq).orEmpty() + new?.supers?.get(fq).orEmpty()) classFile[sup.substringBefore('<')]?.let(out::add)
            }
            if ((k in o) != (k in n) && classFile[k.scope] == null) declIndex[plainSym(k)]?.let(out::addAll)
        }
        out.remove(f)
        return out
    }
    val snapSurfaces = HashMap<String, FileSurface>() // surface each snapshot binary was compiled from
    val index = LookupIndex()
    val keep = LinkedHashSet<String>()
    var snap: Snapshot? = null
    val st = WarmState(reuseEnv = o["reuseEnv"] != "false", reuseLibs = o["reuseLibs"] != "false").also { it.pinLvs = o["pinLvs"] != "false"; it.freshJavaComponents = o["freshJava"] != "false" }
    var tmpSeq = 0

    fun log(s: String) { println(s); logFile?.appendText(s + "\n") }

    class Res(val out: CheckOut, val lookups: CompileLookups, val surfaces: Map<String, FileSurface>, val ms: Long, val tmp: File?, val t: Timings)

    private fun compile(sources: Collection<String>, withSnap: Boolean, backend: Boolean, requested: Collection<String> = sources, stock: Boolean = false): Res {
        val lookups = CompileLookups()
        val surf = HashMap<String, FileSurface>()
        val after: (org.jetbrains.kotlin.fir.pipeline.SingleModuleFrontendOutput) -> Unit = { surf.putAll(Surface.of(it)) }
        val tmp = File(dir, "tmp-${tmpSeq++}").also { it.deleteRecursively(); it.mkdirs() }
        val s = snap
        // the warm environment indexes the snapshot dir once; stock compiles only see it when they use it
        val extra = if (s != null && (isWarm || withSnap)) listOf(s.dir.path) else emptyList()
        val args = kritArgs(sources.toList(), g.classpath, tmp, kotlinArgs = kotlinArgs, extraClasspath = extra)
        if (withSnap) args.incrementalCompilation = true
        if (backend) args.reportOutputFiles = true
        val t = Timings()
        val phase = if (isWarm && !stock) {
            st.snapshotDir = s?.dir
            val pre = if (withSnap) s!!.precompiled else null
            if (backend) Phases.warmFull(st, pre, t, after) else Phases.warmFrontend(st, pre, t, after)
        } else {
            if (backend) StockPhases.full(after) else StockPhases.frontend(after)
        }
        val svc = services(if (track) lookups else null, if (withSnap && (!isWarm || stock)) s else null)
        val t0 = System.nanoTime()
        val out = runCheck(phase, args, requested, svc)
        val d = System.nanoTime() - t0
        if (isWarm && !stock) st.releaseHeldCompiles()
        if (!backend) tmp.deleteRecursively()
        return Res(out, lookups, surf, d, if (backend) tmp else null, t)
    }

    private fun absorb(r: Res, files: Collection<String>) {
        for (f in files) {
            r.out.verdicts[f]?.let { cache[f] = it }
            index.update(f, r.lookups.byFile[f] ?: emptySet())
        }
    }

    private fun subclassesOf(): Map<String, Set<String>> {
        val m = HashMap<String, MutableSet<String>>()
        for (fs in surfaces.values) for ((c, sups) in fs.supers) for (s in sups) m.getOrPut(s.substringBefore('<')) { HashSet() }.add(c)
        return m
    }

    /** Lookup symbols a surface change dirties: the symbols, plus members re-reached through subclasses. */
    private fun expand(syms: Set<Sym>, scopes: Set<String>): Pair<Set<Sym>, Set<String>> {
        if (syms.none { it.scope.isNotEmpty() }) return syms to scopes
        val subs = subclassesOf()
        val outSyms = HashSet(syms); val outScopes = HashSet(scopes)
        fun walk(c: String, f: (String) -> Unit, seen: MutableSet<String> = HashSet()) {
            for (s in subs[c].orEmpty()) if (seen.add(s)) { f(s); walk(s, f, seen) }
        }
        for (s in syms) walk(s.scope, { sub -> outSyms += Sym(sub, s.name) })
        for (sc in scopes) walk(sc, { sub -> outScopes += sub })
        return outSyms to outScopes
    }

    data class EditStat(
        val i: Int, val kind: String, val ms: Long, val cpu: Long, val waves: Int, val compiled: Int, val flagged: Int,
        val fallback: Boolean, val load: String, val waveMs: List<Long>, val keep: Int,
    )

    val stats = ArrayList<EditStat>()
    val quality = ArrayList<String>()
    var staleVerdictEdits = 0; var staleVerdictFiles = 0; var staleStrictEdits = 0; var staleStrictFiles = 0; var compared = 0
    val strictCauses = java.util.TreeMap<String, Int>()

    fun run() {
        dir.deleteRecursively(); dir.mkdirs()
        g = makeProj(o, File(dir, "src"), n, seed)
        log("# ${g.describe()}")
        log("# inc strategy=$strategy snap=$snapMode files=$n edits=$edits warmup=$warmup oracle=$oracleEvery threshold=$threshold track=$track heapMax=${Runtime.getRuntime().maxMemory() shr 20}MB load=${load1()}")
        if (isSlice) snap = Snapshot(File(dir, "snapshot").also { it.mkdirs() }, "main").also { it.virtual = isWarm }
        if (isWarm && isSlice && snapMode == "patch") error("warmslice supports --snap=keep only: patching would change files a reused environment has cached")
        val all0 = g.texts.keys.map(::canon)

        // cold: one full compile establishes verdicts, lookups, surfaces (and, for slice strategies, the snapshot)
        var t0 = System.nanoTime(); val c0 = cpuNanos()
        val cold = if (isSlice) coldSnapshot(all0) else compile(all0, withSnap = false, backend = false)
        absorb(cold, all0); for ((f, sf) in cold.surfaces) setSurface(f, sf); snapSurfaces.putAll(cold.surfaces)
        log("cold: ${ms(System.nanoTime() - t0)} ms cpu=${ms(cpuNanos() - c0)} exit=${cold.out.exit} findings=${cache.values.sumOf { it.findings.size }} errFiles=${cache.values.count { it.error }} lookups=${cold.lookups.records} idxEdges=${index.edges} snapshotClasses=${snap?.classFiles} [${cold.t}]")
        var prevRef: CheckOut? = null
        var prevDeps: Map<String, Set<String>>? = null
        if (oracleEvery > 0) {
            val (ref, deps) = reference(all0)
            prevRef = ref; prevDeps = deps
            val vd = Compare.verdictDiff(ref.verdicts, cache, all0)
            log("cold vs reference (shipped frontend-only): verdictDiff=${vd.size} strictDiff=${Compare.strictDiff(ref.verdicts, cache, all0).size}")
            Compare.strictDiff(ref.verdicts, cache, all0).take(4).forEach { log("   " + Compare.explain(it, ref.verdicts[it], cache[it])) }
            // the reference is what Go would have cached; incremental state starts from it
            cache.putAll(ref.verdicts)
        }
        log("mem after cold: ${memSnap(true)}")

        for (i in 1..(warmup + edits)) {
            val e = g.nextEdit(kinds)
            val E = (e.changed + e.added).filter { File(it).exists() }.map(::canon).distinct()
            val D = e.deleted.map(::canon)
            val tStart = System.nanoTime(); val cStart = cpuNanos(); val l = load1()
            val (compiled, waveMs, waves, fallback) = if (isSlice) sliceEdit(E, D) else fullEdit(D)
            val total = System.nanoTime() - tStart
            val cpu = cpuNanos() - cStart
            val flagged = if (isSlice) lastDependents else compiled - E.toSet()
            lastDependents = emptySet()
            val stat = EditStat(i, e.kind, total, cpu, waves, compiled.size, flagged.size, fallback, l, waveMs.map { ms(it) }, keep.size)
            var line = "e$i ${e.kind} E=${E.size} D=${D.size} waves=$waves compiled=${compiled.size} ${ms(total)} ms cpu=${ms(cpu)} waveMs=${stat.waveMs} keep=${keep.size}${if (fallback) " FALLBACK" else ""} load=$l"
            if (oracleEvery > 0 && i % oracleEvery == 0) {
                val all = g.texts.keys.map(::canon)
                val (ref, deps) = reference(all)
                val sample = if (oracleSample > 0) (compiled + all.shuffled(java.util.Random(i.toLong())).take(oracleSample)).toSet() else all.toSet()
                val vd = Compare.verdictDiff(ref.verdicts, cache, sample)
                val sd = Compare.strictDiff(ref.verdicts, cache, sample)
                compared++
                if (vd.isNotEmpty()) { staleVerdictEdits++; staleVerdictFiles += vd.size }
                if (sd.isNotEmpty()) { staleStrictEdits++; staleStrictFiles += sd.size }
                for (f in sd) for (dd in (ref.verdicts[f]?.diags.orEmpty() + cache[f]?.diags.orEmpty()) - (ref.verdicts[f]?.diags.orEmpty().toSet() intersect cache[f]?.diags.orEmpty().toSet())) {
                    strictCauses.merge(Regex("\\[([A-Za-z_]+)]|^.\\|[^|]*\\|([A-Za-z ]{0,40})").find(dd)?.value?.take(40) ?: "?", 1, Int::plus)
                }
                line += " | ref: verdictStale=${vd.size} strictStale=${sd.size}"
                vd.take(3).forEach { line += "\n     STALE " + Compare.explain(it, ref.verdicts[it], cache[it]) }
                if (vd.isEmpty()) sd.take(2).forEach { line += "\n     strict " + Compare.explain(it, ref.verdicts[it], cache[it]) }
                // dependents quality: files (not edited) whose fresh verdict changed with this edit
                val pr = prevRef
                if (pr != null && oracleSample == 0) {
                    val truthStrict = all.filter { it !in E && pr.verdicts[it] != ref.verdicts[it] }.toSet()
                    val truthVerdict = all.filter { it !in E && pr.verdicts[it]?.verdictKey() != ref.verdicts[it]?.verdictKey() }.toSet()
                    // flagged = dependents the lookup index chose; covered = every file re-checked (incl. stale-binary files)
                    val covered = compiled - E.toSet()
                    val missed = truthStrict - covered
                    var q = "${e.kind}|flagged=${flagged.size}|truth=${truthStrict.size}|truthVerdict=${truthVerdict.size}|missed=${missed.size}|missedVerdict=${(truthVerdict - covered).size}|missedByIndex=${(truthStrict - flagged).size}"
                    val pd = prevDeps
                    if (pd != null) {
                        val changedSet = (E + D).toSet()
                        val depDirty = pd.filter { (f, ds) -> f !in changedSet && ds.any { it in changedSet } }.keys
                        q += "|depTracker=${depDirty.size}|depTrackerMissed=${(truthStrict - depDirty).size}"
                    }
                    quality += q
                    line += " | $q"
                    if (missed.isNotEmpty()) line += "\n     MISSED ${missed.take(4).map { File(it).name }}"
                }
                prevRef = ref; prevDeps = deps
                // keep the harness honest: resync stale entries so one miss is not counted on every later edit
                if (o["resync"] == "true") cache.putAll(ref.verdicts)
            }
            log(line)
            if (i > warmup) stats += stat
            if (gcEvery > 0 && i % gcEvery == 0) log("mem@e$i: ${memSnap(true)} idxFiles=${index.files} idxEdges=${index.edges} snapshotClasses=${snap?.classFiles} envBuilds=${st.envBuilds} libBuilds=${st.libBuilds}")
        }
        summary()
        st.dispose()
        if (o["keepDir"] != "true") dir.deleteRecursively()
    }

    private fun coldSnapshot(all: List<String>): Res {
        val r = compile(all, withSnap = false, backend = true, stock = true)
        val s = snap!!
        // the backend wrote into a temp dir; that becomes the snapshot
        s.dir.deleteRecursively()
        val tmp = r.tmp!!
        if (!tmp.renameTo(s.dir)) error("cannot move $tmp to ${s.dir}")
        val outs = r.out.outputs.mapValues { (_, ps) -> ps.map { it.replace(tmp.path, s.dir.path) } }
        s.adopt(outs)
        log("snapshot: ${s.classFiles} class files for ${outs.size} sources (mapping ${s.mappingBytes()?.size ?: 0} bytes); backend ran=${r.out.exit}")
        // sources without outputs (errors: the backend never ran) can never come from the snapshot
        for (f in all) if (f !in s.outputs) keep += f
        return r
    }

    /** Fresh shipped frontend-only compile of the whole module; optionally also krit-fir's DepTracker edges. */
    private fun reference(all: List<String>): Pair<CheckOut, Map<String, Set<String>>?> {
        val tmp = File(dir, "ref").also { it.deleteRecursively(); it.mkdirs() }
        val args = kritArgs(all, g.classpath, tmp, kotlinArgs = kotlinArgs)
        if (!depTracker) return runShipped(args, all, frontendOnly = true) to null
        val coll = OracleCollector()
        OracleCollectorRegistry.begin(coll)
        val out = try { runShipped(args, all, frontendOnly = true) } finally { OracleCollectorRegistry.end() }
        val deps = coll.depTracker.depPathsByFile.entries.associate { (k, v) -> canon(k) to v.map(::canon).toSet() }
        return out to deps
    }

    private data class EditResult(val compiled: Set<String>, val waveMs: List<Long>, val waves: Int, val fallback: Boolean)

    private fun fullEdit(D: List<String>): EditResult {
        for (d in D) { cache.remove(d); index.remove(d); setSurface(d, null) }
        val all = g.texts.keys.map(::canon)
        val r = compile(all, withSnap = false, backend = false)
        absorb(r, all); for ((f, sf) in r.surfaces) setSurface(f, sf)
        return EditResult(all.toSet(), listOf(r.ms), 1, false)
    }

    private fun sliceEdit(E: List<String>, D: List<String>): EditResult {
        val s = snap!!
        var (syms, scopes) = Pair(HashSet<Sym>(), HashSet<String>())
        val extra = HashSet<String>()
        for (d in D) {
            val (a, b) = FileSurface.diff(surfaces[d], null); syms += a; scopes += b
            extra += structural(d, surfaces[d], null)
            setSurface(d, null); snapSurfaces.remove(d); cache.remove(d); index.remove(d); keep.remove(d)
            s.drop(listOf(d))
        }
        val keptBefore = keep.toSet()
        // literal option (b): every file edited since the snapshot stays in the source set
        if (snapMode == "keepall") keep += E
        val compiled = LinkedHashSet<String>()
        val waveMs = ArrayList<Long>()
        val (es, esc) = expand(syms, scopes)
        val found = HashSet<String>(index.dependents(es, esc) + extra)
        var todo: Set<String> = (E + found).toSet() - D.toSet()
        var waves = 0
        while (todo.isNotEmpty()) {
            if (waves >= maxWaves || compiled.size + todo.size > threshold) {
                // fallback: full frontend compile of the module, snapshot binaries not used
                val all = g.texts.keys.map(::canon)
                val r = compile(all, withSnap = false, backend = false)
                absorb(r, all)
                for ((f, sf) in r.surfaces) { setSurface(f, sf); if (snapSurfaces[f] != sf) keep += f }
                waveMs += r.ms
                lastDependents = found - E.toSet() - D.toSet()
                return EditResult(all.toSet(), waveMs, waves + 1, true)
            }
            waves++
            val sources = (todo + keep).filter { File(it).exists() }.toCollection(LinkedHashSet())
            s.park(sources)
            val backend = snapMode == "patch"
            val r = compile(sources, withSnap = true, backend = backend)
            waveMs += r.ms
            absorb(r, sources)
            syms = HashSet(); scopes = HashSet()
            val reach = HashSet<String>()
            for (f in sources) {
                val nsf = r.surfaces[f]
                val (a, b) = FileSurface.diff(surfaces[f], nsf); syms += a; scopes += b
                reach += structural(f, surfaces[f], nsf)
                if (nsf != null) setSurface(f, nsf)
            }
            val patched = backend && r.out.exit == org.jetbrains.kotlin.cli.common.ExitCode.OK && r.out.outputs.isNotEmpty()
            if (patched) {
                s.patch(r.tmp!!, r.out.outputs, sources)
                for (f in sources) { snapSurfaces[f] = surfaces[f] ?: continue; keep.remove(f) }
            } else {
                // A binary is stale for the frontend only if the file's surface changed (bodies never reach other files'
                // resolution, except inline bodies / const values / contracts, which are part of the surface).
                for (f in sources) if (snapSurfaces[f] != surfaces[f] || f !in s.outputs) keep += f
                s.unpark(drop = keep)
            }
            r.tmp?.deleteRecursively()
            compiled += sources
            val (xs, xsc) = expand(syms, scopes)
            val deps = index.dependents(xs, xsc) + reach; found += deps
            todo = deps - compiled - D.toSet()
        }
        lastDependents = found - E.toSet() - D.toSet()
        return EditResult(compiled, waveMs, waves, false)
    }
    var lastDependents: Set<String> = emptySet()

    private fun summary() {
        val tot = stats.map { it.ms * 1_000_000 }
        log("## $strategy/$snapMode n=${stats.size} p50=${ms(pct(tot, .5))} p95=${ms(pct(tot, .95))} max=${ms(tot.maxOrNull() ?: 0)} ms; cpu p50=${pct(stats.map { it.cpu }, .5) / 1_000_000} ms; fallbacks=${stats.count { it.fallback }}; loads=${stats.map { it.load }.let { "${it.minOrNull()}-${it.maxOrNull()}" }}")
        for (k in stats.map { it.kind }.distinct().sorted()) {
            val x = stats.filter { it.kind == k }
            log("  %-9s n=%3d p50=%6d p95=%6d max=%6d ms  compiled p50=%d max=%d waves max=%d".format(
                k, x.size, pct(x.map { it.ms }, .5) / 1_000_000, pct(x.map { it.ms }, .95) / 1_000_000, x.maxOf { it.ms } / 1_000_000,
                pct(x.map { it.compiled.toLong() }, .5), x.maxOf { it.compiled }, x.maxOf { it.waves }))
        }
        if (compared > 0) log("## correctness: compared=$compared verdictStale edits=$staleVerdictEdits files=$staleVerdictFiles; strictStale edits=$staleStrictEdits files=$staleStrictFiles causes=$strictCauses")
        if (quality.isNotEmpty()) {
            val byKind = quality.groupBy { it.substringBefore('|') }
            for ((k, qs) in byKind.toSortedMap()) {
                fun avg(key: String) = qs.map { q -> q.split('|').first { it.startsWith("$key=") }.substringAfter('=').toDouble() }.average()
                fun sum(key: String) = qs.sumOf { q -> q.split('|').firstOrNull { it.startsWith("$key=") }?.substringAfter('=')?.toInt() ?: 0 }
                val dt = if (qs.first().contains("depTracker=")) " depTracker avg=%.1f missed=%d".format(avg("depTracker"), sum("depTrackerMissed")) else ""
                log("  quality %-9s n=%3d flagged avg=%.1f truth avg=%.1f (verdict %.1f) missed=%d missedVerdict=%d (index alone missed %d)%s".format(
                    k, qs.size, avg("flagged"), avg("truth"), avg("truthVerdict"), sum("missed"), sum("missedVerdict"), sum("missedByIndex"), dt))
            }
        }
        log("## memory at end: ${memSnap(true)}")
        if (o["histo"] == "true") histogram(o["histoTop"]?.toInt() ?: 45).forEach { log("  histo $it") }
    }
}
