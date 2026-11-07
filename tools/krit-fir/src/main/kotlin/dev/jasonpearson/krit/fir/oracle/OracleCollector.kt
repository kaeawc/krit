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
    private val diagnosticCharRanges = HashMap<String, MutableList<Pair<Int, Int>>>()
    private val smartCastsByFile = HashMap<String, MutableList<SmartCastDeclarations>>()
    private val errorTaintedByFile = HashMap<String, MutableList<Pair<Int, Int>>>()
    private val errorTaintedStatementsByFile = HashMap<String, MutableList<Pair<Int, Int>>>()
    private val errorTaintedCalleesByFile = HashMap<String, MutableList<Pair<Int, Int>>>()
    private val shadowedContractSymbols = HashSet<Any>()
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

    /**
     * Slice compiles only: the module file declaring a class the compile read as a binary from the incremental
     * snapshot, so a smart cast on its property still names the file its source would have. Null outside slices.
     */
    var binaryClassFile: ((fqn: String) -> String?)? = null

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
     * Add a call payload for [filePath] under [key] ("line:col"). Calls of a chain such as `a.b().c()`
     * share their start, so a call whose byte range differs from the entry already at [key] goes under
     * [rangeKey] instead; readers keyed on "line:col" alone skip that form. A call already recorded
     * with the same range is kept (first-wins).
     */
    fun addCallExpression(filePath: String, key: String, payload: ExpressionPayload) {
        val map = expressionsByFile.getOrPut(filePath) { LinkedHashMap() }
        val existing = map[key]
        if (existing == null) {
            map[key] = payload
            return
        }
        if (existing.sameRange(payload) || payload.endByte <= payload.startByte) return
        map.putIfAbsent(rangeKey(key, payload.endByte), payload)
    }

    /** True iff an expression spanning [startByte, endByte) is already recorded at [key] or its [rangeKey]. */
    fun hasExpressionRange(filePath: String, key: String, startByte: Int, endByte: Int): Boolean {
        val map = expressionsByFile[filePath] ?: return false
        val existing = map[key] ?: return false
        return (existing.startByte == startByte && existing.endByte == endByte) || map.containsKey(rangeKey(key, endByte))
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
        // A call such as `x.foo()` shares its start with the smart-cast receiver `x`, which then lost its refined
        // type to the call at [key]. Readers that look a node up by byte range find this copy instead.
        if (existing != null && payload.endByte > payload.startByte && !existing.sameRange(payload)) {
            map[rangeKey(key, payload.endByte)] = payload
        }
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
    fun addDiagnostic(filePath: String, payload: DiagnosticPayload, startChar: Int = -1, endChar: Int = -1) {
        diagnosticsByFile.getOrPut(filePath) { mutableListOf() }.add(payload)
        diagnosticCharRanges.getOrPut(filePath) { mutableListOf() }.add(startChar to endChar)
    }

    /**
     * Record that the stable smart cast spanning chars [startChar, endChar) of [filePath] relies on properties
     * declared in [files] (other source files). [toResult] attaches them to the smart-cast-dependent diagnostics
     * that smart cast decides, whichever order FIR reports the two in.
     */
    fun addSmartCastDeclarations(filePath: String, startChar: Int, endChar: Int, files: Set<String>) {
        if (files.isEmpty()) return
        smartCastsByFile.getOrPut(filePath) { mutableListOf() }.add(SmartCastDeclarations(startChar, endChar, files))
    }

    /**
     * Record that chars [startChar, endChar) of [filePath] are the operand of a safe call, `!!`, elvis, cast or
     * comparison whose type derives from an unresolved or error-typed expression. The compiler treats an error type
     * as non-null and as a subtype of everything, so the verdict on that operator is unreliable.
     */
    fun addErrorTainted(filePath: String, startChar: Int, endChar: Int) {
        if (startChar < 0 || endChar < startChar) return
        errorTaintedByFile.getOrPut(filePath) { mutableListOf() }.add(startChar to endChar)
    }

    /**
     * Record [symbol], a variable passed to a call whose name an unresolved import or an unresolved supertype may
     * have bound to a different function. If the compiler picked a contract function of that name instead, the
     * smart cast it made on [symbol] describes that guess, not the program.
     */
    fun addShadowedContractSymbol(symbol: Any) {
        shadowedContractSymbols.add(symbol)
    }

    /** Whether [symbol] was passed to a call recorded by [addShadowedContractSymbol]. */
    fun isShadowedContractSymbol(symbol: Any): Boolean = symbol in shadowedContractSymbols

    /**
     * Record that chars [startChar, endChar) of [filePath] are a statement (or a local's initializer) whose type derives
     * from an error type. The compiler treats an error type as `Nothing`, so it marks whatever follows as unreachable.
     */
    fun addErrorTaintedStatement(filePath: String, startChar: Int, endChar: Int) {
        if (startChar < 0 || endChar < startChar) return
        errorTaintedStatementsByFile.getOrPut(filePath) { mutableListOf() }.add(startChar to endChar)
    }

    /**
     * Record that chars [startChar, endChar) of [filePath] are the name of a callee that was resolved through an
     * error-typed receiver or argument, so the declaration the compiler picked (and its deprecation) is a guess.
     */
    fun addErrorTaintedCallee(filePath: String, startChar: Int, endChar: Int) {
        if (startChar < 0 || endChar < startChar) return
        errorTaintedCalleesByFile.getOrPut(filePath) { mutableListOf() }.add(startChar to endChar)
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
                diagnostics = diagnosticsWithSmartCasts(path),
                lambdaSuspendByLineCol = lambdaSuspendByFile[path]?.toMap() ?: emptyMap(),
            )
        }
        return AnalyzeResult(files = files, dependencies = dependencies.toMap())
    }

    private fun diagnosticsWithSmartCasts(path: String): List<DiagnosticPayload> {
        val all = diagnosticsByFile[path] ?: return emptyList()
        val allRanges = diagnosticCharRanges[path]
        val offsets = offsetsFor(path)
        val keep = all.indices.filter { i ->
            val (start, end) = allRanges?.get(i) ?: return@filter true
            offsets == null || start < 0 || !isErrorDerived(path, all[i].factoryName, start, end, offsets)
        }
        val diagnostics = keep.map { all[it] }
        val ranges = allRanges?.let { r -> keep.map { r[it] } }
        val smartCasts = smartCastsByFile[path]
        if (smartCasts == null || ranges == null || offsets == null) return diagnostics
        return diagnostics.mapIndexed { i, d ->
            val (start, end) = ranges[i]
            if (d.factoryName !in SMART_CAST_FACTORIES || start < 0) return@mapIndexed d
            val files = smartCasts.filter { it.decides(d.factoryName, start, end, offsets) }
                .flatMapTo(sortedSetOf()) { it.files }
            if (files.isEmpty()) d else d.copy(smartCastDeclarations = files.toList())
        }
    }

    /**
     * Whether the [factory] verdict at chars [start, end) of [path] rests on an error type: its operand, the code
     * before it, or the callee it names was typed or resolved from a class the compilation could not see.
     */
    private fun isErrorDerived(path: String, factory: String, start: Int, end: Int, offsets: FileOffsetTable): Boolean =
        when (factory) {
            in OPERAND_FACTORIES ->
                errorTaintedByFile[path]?.any { (s, e) -> decidedBy(factory, s, e, start, end, offsets) } == true
            "UNREACHABLE_CODE" ->
                errorTaintedStatementsByFile[path]?.any { (_, e) -> e <= start && offsets.onlyStatementGapBetween(e, start) } == true
            "DEPRECATION" ->
                errorTaintedCalleesByFile[path]?.any { (s, e) -> start >= s && end <= e } == true
            else -> false
        }

    private class SmartCastDeclarations(val startChar: Int, val endChar: Int, val files: Set<String>) {
        /**
         * Whether this smart cast decides a [factory] diagnostic at chars [start, end). A null comparison
         * contains its operand; the postfix verdicts (`?.`, `!!`, `?:`, `as`) start right after their receiver.
         */
        fun decides(factory: String, start: Int, end: Int, offsets: FileOffsetTable): Boolean =
            decidedBy(factory, startChar, endChar, start, end, offsets)
    }

    companion object {
        /**
         * Whether the operand at chars [operandStart, operandEnd) decides a [factory] diagnostic at chars
         * [start, end). A null comparison contains its operand; the postfix verdicts (`?.`, `!!`, `?:`, `as`)
         * start right after their receiver.
         */
        fun decidedBy(factory: String, operandStart: Int, operandEnd: Int, start: Int, end: Int, offsets: FileOffsetTable): Boolean =
            if (factory == "SENSELESS_COMPARISON") operandStart >= start && operandEnd <= end
            else operandEnd <= start && offsets.onlyClosingBetween(operandEnd, start)

        /** The retained factories whose verdict can rest on a smart cast's stability. */
        val SMART_CAST_FACTORIES = setOf(
            "UNNECESSARY_SAFE_CALL", "UNNECESSARY_NOT_NULL_ASSERTION", "USELESS_ELVIS", "SENSELESS_COMPARISON", "USELESS_CAST",
        )

        /** The retained factories decided by an operand: a null-safety verdict or a cast. */
        val OPERAND_FACTORIES = SMART_CAST_FACTORIES + "CAST_NEVER_SUCCEEDS"
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
