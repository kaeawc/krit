package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import java.io.File

// Research plumbing for the tiered-rules study (#740, tiered-rules-findings.md). Nothing here is production code: it
// computes lookup closures over an IncrementalState, the per-rule trigger gate the study evaluates, and per-file
// verdict splits into tiered and non-tiered rules.

/** Rules whose verdict reads other declarations' bodies or the compilation's files. */
internal val tieredRuleIds: Set<String> by lazy { FirRuleDiscovery.wholeCompilationRuleIds().toSet() }

/** One file's findings split into tiered-rule and other findings; the error gate and rule errors ride with "other". */
internal data class SplitVerdict(val tiered: List<String>, val other: List<String>, val error: Boolean, val ruleErrors: Set<String>)

internal fun BatchResult.splitVerdictOf(path: String): SplitVerdict {
    val v = verdictOf(path)
    val (t, o) = v.findings.partition { it.substringBefore('@') in tieredRuleIds }
    return SplitVerdict(t, o, v.error, v.ruleErrors)
}

/**
 * Lookup-based dependency closures over [state]: file F depends on file G when F's last compile looked up a symbol G
 * declares (a top-level name, a class, or a member of a class G declares). This is the "files whose bodies a tiered
 * rule on F can read" approximation of option A, at hop depth k.
 */
internal class LookupClosure(private val state: IncrementalState) {
    val files: Set<String> = state.files()
    private val deps = HashMap<String, Set<String>>()

    fun depsOf(file: String): Set<String> = deps.getOrPut(file) {
        val out = HashSet<String>()
        for (sym in state.lookupsOf(file)) out += state.filesDeclaring(sym)
        out -= file
        out
    }

    /** Files reachable from [roots] in at most [k] hops (k = Int.MAX_VALUE: transitive), roots excluded. */
    fun closure(roots: Collection<String>, k: Int): Set<String> {
        val seen = HashSet(roots)
        var frontier: Collection<String> = roots
        var hop = 0
        while (frontier.isNotEmpty() && hop < k) {
            val next = ArrayList<String>()
            for (f in frontier) for (d in depsOf(f)) if (seen.add(d)) next += d
            frontier = next
            hop++
        }
        return seen - roots.toSet()
    }

    /** Hop distance from [root] to each reachable file. */
    fun hops(root: String): Map<String, Int> {
        val out = HashMap<String, Int>()
        out[root] = 0
        val queue = ArrayDeque(listOf(root))
        while (queue.isNotEmpty()) {
            val f = queue.removeFirst()
            for (d in depsOf(f)) if (d !in out) { out[d] = out.getValue(f) + 1; queue.add(d) }
        }
        return out
    }

    /** Files F (other than [changed]) whose closure at depth [k] contains a changed file: their tiered verdict may be stale. */
    fun affected(changed: Collection<String>, k: Int): Set<String> {
        val changedSet = changed.toSet()
        val out = HashSet<String>()
        for (f in files) {
            if (f in changedSet) continue
            if (depsWithin(f, k).any { it in changedSet }) out += f
        }
        return out
    }

    private val withinCache = HashMap<Pair<String, Int>, Set<String>>()
    private fun depsWithin(f: String, k: Int): Set<String> = withinCache.getOrPut(f to k) { closure(listOf(f), k) }
}

/**
 * The trigger gate the study evaluates: a tiered rule can only report on file F when F's own resolution reaches the
 * rule's entry point, which the compiler's LookupTracker records as a (scope, name) lookup in F. Rules whose evidence
 * sits in the *other* file (a wrapper val, a factory body) are gated by a "seed" lookup in a file of F's closure instead.
 * Name-only patterns (any scope) are used where the entry is a project declaration's name.
 */
internal object TieredTriggers {
    private val sinks: Map<String, List<Pair<String?, Set<String>>>> = mapOf(
        "HardcodedSecretKey" to listOf("javax.crypto.spec" to setOf("SecretKeySpec")),
        "StaticIv" to listOf("javax.crypto.spec" to setOf("IvParameterSpec", "GCMParameterSpec")),
        "WeakKeySize" to listOf("javax.crypto.KeyGenerator" to setOf("init"), "java.security.KeyPairGenerator" to setOf("initialize")),
        "LongLogTag" to listOf("android.util.Log" to setOf("v", "d", "i", "w", "e", "wtf")),
        "LocaleGetDefaultForFormatting" to listOf(
            "java.time.format.DateTimeFormatter" to setOf("withLocale"),
            "org.threeten.bp.format.DateTimeFormatter" to setOf("withLocale"),
        ),
        "JdbcStatementExecute" to listOf("java.sql.Statement", "java.sql.PreparedStatement", "java.sql.CallableStatement").map {
            it to setOf("execute", "executeQuery", "executeUpdate", "executeLargeUpdate")
        },
        "ShortAlarm" to listOf(null to setOf("setRepeating", "setInexactRepeating")),
        "WorldWriteableFiles" to listOf(null to setOf("MODE_WORLD_WRITEABLE", "MODE_WORLD_WRITABLE")),
        "SecureRandom" to listOf("java.util.Random" to setOf("setSeed"), "java.security.SecureRandom" to setOf("setSeed")),
        "ShowToast" to listOf("android.widget.Toast" to setOf("makeText", "show")),
        "GetSignatures" to listOf("android.content.pm.PackageManager" to setOf("getPackageInfo", "getPackageArchiveInfo", "getInstalledPackages", "GET_SIGNATURES")),
        "GlobalScopeLaunchInViewModel" to listOf("kotlinx.coroutines" to setOf("launch", "async")),
    )

    // Evidence lives in the declaration another file holds; F is gated when its closure holds a file with this lookup.
    private val seeds: Map<String, List<Pair<String?, Set<String>>>> = mapOf(
        "CollectionsSynchronizedListIteration" to listOf("java.util.Collections" to setOf(
            "synchronizedList", "synchronizedCollection", "synchronizedSet", "synchronizedMap", "synchronizedSortedSet",
            "synchronizedSortedMap", "synchronizedNavigableSet", "synchronizedNavigableMap",
        )),
        "SharedFlowWithoutReplay" to listOf("kotlinx.coroutines.flow" to setOf("MutableSharedFlow")),
    )

    private fun matches(syms: Set<Sym>, patterns: List<Pair<String?, Set<String>>>): Boolean =
        syms.any { s -> patterns.any { (scope, names) -> (scope == null || s.scope == scope) && s.name in names } }

    /** Tiered rules whose sink lookup [file] has. */
    fun sinkRules(state: IncrementalState, file: String): Set<String> {
        val syms = state.lookupsOf(file)
        return sinks.filterValues { matches(syms, it) }.keys
    }

    fun hasSeed(state: IncrementalState, file: String): Set<String> {
        val syms = state.lookupsOf(file)
        return seeds.filterValues { matches(syms, it) }.keys
    }

    /**
     * True when a tiered rule may need other files' bodies for [file]: [file] has a sink lookup, or a file within
     * [k] hops of it (or itself) has a seed lookup. SerialVersionUIDInSerializableClass and TooGenericExceptionThrown
     * are not gated here (see the findings: one needs a module-wide name index, the other a module-origin check).
     */
    fun gated(state: IncrementalState, closure: LookupClosure, file: String, k: Int): Boolean {
        if (sinkRules(state, file).isNotEmpty()) return true
        if (hasSeed(state, file).isNotEmpty()) return true
        return closure.closure(listOf(file), k).any { hasSeed(state, it).isNotEmpty() }
    }

    val gatedRules: Set<String> get() = sinks.keys + seeds.keys
}

internal fun pct(xs: List<Long>, p: Double): Long {
    if (xs.isEmpty()) return -1
    val s = xs.sorted()
    return s[((s.size - 1) * p).toInt().coerceIn(0, s.size - 1)]
}

internal fun pctI(xs: List<Int>, p: Double): Int {
    if (xs.isEmpty()) return -1
    val s = xs.sorted()
    return s[((s.size - 1) * p).toInt().coerceIn(0, s.size - 1)]
}

/** The shared Kotlin stubs of the compiler tests, compiled once into a class directory; Java stubs are source roots. */
internal object CompilerTestStubs {
    val data = File("compiler-tests/src/test/data")
    val javaRoot: String get() = File(data, "stubs/java").path

    fun compile(work: File): File {
        val out = File(work, "kt-stubs").also { it.mkdirs() }
        val stubSources = File(data, "stubs").listFiles { f -> f.extension == "kt" }!!.map { it.path }.sorted()
        compileKotlinLibrary(stubSources, out, listOf(javaRoot))
        check(out.walkTopDown().any { it.extension == "class" }) { "stub library did not compile" }
        return out
    }

    fun session(src: List<String>, cp: List<String>, record: Boolean, snapshots: File? = null): AnalysisSession =
        AnalysisSession(src, cp, "", emptyList(), incrementalRecording = record).also {
            if (snapshots != null) it.snapshotRootOverride = snapshots
            it.argumentsHook = { args -> args.javaSourceRoots = arrayOf(javaRoot) }
        }
}
