package dev.jasonpearson.krit.fir.oracle

import java.io.File

/**
 * Mutable buffer that collects per-class projections during a single
 * K2 compilation. The orchestrator ([`AnalysisSession.analyze`]) creates
 * one [OracleCollector], publishes it on [OracleCollectorRegistry] for
 * the duration of compilation, then drains it once the compiler returns.
 *
 * The compiler walks declarations on a single thread per compilation,
 * so the collector itself is not synchronized. [OracleCollectorRegistry]
 * uses a thread-local to scope visibility — multiple concurrent
 * compilations on different threads each see their own collector.
 */
internal class OracleCollector {
    private val perFile = LinkedHashMap<String, MutableList<ClassPayload>>()
    private val dependencies = LinkedHashMap<String, ClassPayload>()
    private val packageByFile = LinkedHashMap<String, String>()
    private val expressionsByFile = LinkedHashMap<String, LinkedHashMap<String, ExpressionPayload>>()
    private val diagnosticsByFile = LinkedHashMap<String, MutableList<DiagnosticPayload>>()
    private val lambdaSuspendByFile = LinkedHashMap<String, LinkedHashMap<String, Boolean>>()
    private val offsetsByFile = HashMap<String, FileOffsetTable?>()

    /**
     * Per-file dependency-closure tracker. Populated by
     * [OracleClassChecker] when it walks supertypes resolvable to
     * other source files in the same compilation; drained by
     * [OracleResponse.buildAnalyzeWithDeps] to populate the
     * `cacheDeps` field on the wire.
     */
    val depTracker: DepTracker = DepTracker()

    /**
     * Slice compiles only (#740): resolves a supertype the compile read as a binary from the incremental snapshot
     * to the module file declaring it and that class's projection, as a whole-module compile would have recorded
     * them from source. Returns null for a class that is not the module's (a library class). Null outside slices.
     */
    var binarySupertype: ((fqn: String) -> Pair<String, ClassPayload>?)? = null

    /** Set when [binarySupertype] knew the declaring file but had no projection: this compile's deps are partial. */
    var binarySupertypeMissed: Boolean = false

    /**
     * Slice compiles only (#740): the source-side annotations of a callee the compile read as a binary, by
     * [calleeSignature]; null when the module's source declares none for it (or it is not the module's).
     */
    var binaryCalleeAnnotations: ((signature: String) -> List<String>?)? = null

    /** Set when a binary callee's key was [AMBIGUOUS_ANNOTATIONS]: this compile's facts are not trusted. */
    var binaryCalleeAmbiguous: Boolean = false

    /** Whether to record [declaredAnnotations]: on while the session keeps oracle facts for incremental use. */
    var recordDeclarations: Boolean = false

    private val declaredByFile = LinkedHashMap<String, LinkedHashMap<String, List<String>>>()

    /** Per file, the annotations of each annotated function and constructor it declares (see [recordDeclaredAnnotations]). */
    val declaredAnnotations: Map<String, Map<String, List<String>>> get() = declaredByFile

    fun addDeclaredAnnotations(filePath: String, signature: String, annotations: List<String>) {
        val declared = declaredByFile.getOrPut(filePath) { LinkedHashMap() }
        declared[signature] = mergeDeclared(declared[signature], annotations)
    }

    fun addClass(filePath: String, payload: ClassPayload) {
        perFile.getOrPut(filePath) { mutableListOf() }.add(payload)
        // The `dependencies` map is a cross-file class index — keyed
        // by FQN so the Go-side oracle can resolve a symbol without
        // walking every file. First-wins on duplicate FQNs across
        // files, so the Go-side resolver sees a stable choice.
        dependencies.putIfAbsent(payload.fqn, payload)
    }

    fun setPackage(filePath: String, packageName: String) {
        packageByFile[filePath] = packageName
    }

    /**
     * Add an expression payload for [filePath] under [key]
     * ("line:col"). First-wins on duplicate keys, so overlapping call
     * expressions at the same source position are reported once.
     */
    fun addExpression(filePath: String, key: String, payload: ExpressionPayload) {
        val map = expressionsByFile.getOrPut(filePath) { LinkedHashMap() }
        map.putIfAbsent(key, payload)
    }

    /**
     * Record a smart-cast-refined type for [filePath] at [key]. Unlike
     * [addExpression], this is authoritative over a declared-type entry
     * already recorded at the same key, so the result never depends on
     * whether FIR visits the wrapping smart cast before or after its inner
     * access. Later [addExpression] calls can't displace it (first-wins).
     * A call-site entry (one with a call target) stays: a smart cast wraps
     * a stable reference, not a call, and call metadata must not be lost.
     */
    fun addSmartCastExpression(filePath: String, key: String, payload: ExpressionPayload) {
        val map = expressionsByFile.getOrPut(filePath) { LinkedHashMap() }
        val existing = map[key]
        if (existing == null || existing.callTarget == null) map[key] = payload
    }

    /** True iff an expression has already been recorded at this key in this file. */
    fun hasExpression(filePath: String, key: String): Boolean =
        expressionsByFile[filePath]?.containsKey(key) == true

    /**
     * Append a diagnostic payload for [filePath]. Insertion order is
     * preserved so the wire payload is deterministic; diagnostics are
     * not deduplicated (a file can legitimately
     * carry multiple instances of the same factory at different sites).
     */
    fun addDiagnostic(filePath: String, payload: DiagnosticPayload) {
        diagnosticsByFile.getOrPut(filePath) { mutableListOf() }.add(payload)
    }

    /**
     * Record a lambda's suspend status for [filePath] at the source
     * position keyed by [key] ("line:col"). First-wins on duplicate
     * keys — a single source position can only refer to one lambda,
     * and revisits during phased FIR resolution would otherwise let
     * an intermediate (pre-conversion) status overwrite the final one.
     */
    fun addLambdaSuspend(filePath: String, key: String, isSuspend: Boolean) {
        val map = lambdaSuspendByFile.getOrPut(filePath) { LinkedHashMap() }
        map.putIfAbsent(key, isSuspend)
    }

    /**
     * Lazily-built offset table for [filePath]. Returns null if the
     * file cannot be read (e.g. it was deleted between the K2 walk
     * scheduling and the checker firing). Callers should skip the
     * expression in that case rather than crashing.
     */
    fun offsetsFor(filePath: String): FileOffsetTable? =
        offsetsByFile.getOrPut(filePath) {
            runCatching { FileOffsetTable(File(filePath).readText()) }.getOrNull()
        }

    /**
     * Build the final [AnalyzeResult] from buffered data. Files that
     * had no class declarations but did have a package directive still
     * appear with an empty `declarations` list, so there is one file
     * entry per visited Kotlin file.
     */
    fun toResult(): AnalyzeResult {
        val allFilePaths =
            (perFile.keys + packageByFile.keys + expressionsByFile.keys +
                diagnosticsByFile.keys + lambdaSuspendByFile.keys).toSet()
        val files = LinkedHashMap<String, FilePayload>(allFilePaths.size)
        for (path in allFilePaths) {
            files[path] = FilePayload(
                packageName = packageByFile[path] ?: "",
                declarations = perFile[path]?.toList() ?: emptyList(),
                expressions = expressionsByFile[path]?.toMap() ?: emptyMap(),
                diagnostics = diagnosticsByFile[path]?.toList() ?: emptyList(),
                lambdaSuspendByLineCol = lambdaSuspendByFile[path]?.toMap() ?: emptyMap(),
            )
        }
        return AnalyzeResult(files = files, dependencies = dependencies.toMap())
    }
}

/**
 * Thread-local registry that publishes the active [OracleCollector] for
 * the duration of a single K2 compilation. K2's frontend pipeline is
 * single-threaded per compilation, so a thread-local is sufficient to
 * scope collector visibility across the dispatched class checker and
 * the orchestrator.
 */
internal object OracleCollectorRegistry {
    private val active = ThreadLocal<OracleCollector?>()

    fun begin(collector: OracleCollector) {
        active.set(collector)
    }

    fun current(): OracleCollector? = active.get()

    fun end() {
        active.remove()
    }
}
