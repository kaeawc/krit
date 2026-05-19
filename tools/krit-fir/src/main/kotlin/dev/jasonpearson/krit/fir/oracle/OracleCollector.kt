package dev.jasonpearson.krit.fir.oracle

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

    fun addClass(filePath: String, payload: ClassPayload) {
        perFile.getOrPut(filePath) { mutableListOf() }.add(payload)
        // The `dependencies` map mirrors krit-types' cross-file class
        // index — keyed by FQN so the Go-side oracle can resolve a
        // symbol without walking every file. First-wins on duplicate
        // FQNs across files; matches krit-types' behavior.
        dependencies.putIfAbsent(payload.fqn, payload)
    }

    fun setPackage(filePath: String, packageName: String) {
        packageByFile[filePath] = packageName
    }

    /**
     * Build the final [AnalyzeResult] from buffered data. Files that
     * had no class declarations but did have a package directive still
     * appear with an empty `declarations` list — matches krit-types'
     * behavior of emitting one `FileResult` per visited Kotlin file.
     */
    fun toResult(): AnalyzeResult {
        val allFilePaths = (perFile.keys + packageByFile.keys).toSet()
        val files = LinkedHashMap<String, FilePayload>(allFilePaths.size)
        for (path in allFilePaths) {
            files[path] = FilePayload(
                packageName = packageByFile[path] ?: "",
                declarations = perFile[path]?.toList() ?: emptyList(),
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
