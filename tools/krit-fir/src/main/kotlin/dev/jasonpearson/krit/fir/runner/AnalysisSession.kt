package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.oracle.AnalyzeResult
import dev.jasonpearson.krit.fir.oracle.OracleCollector
import dev.jasonpearson.krit.fir.oracle.OracleCollectorRegistry
import dev.jasonpearson.krit.fir.oracle.OracleDiagnosticMessageCollector
import dev.jasonpearson.krit.fir.oracle.OracleResponse
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File
import java.nio.file.Files

data class FileRef(val path: String, val contentHash: String = "")

/**
 * Bundle of an analyze run's structured result and per-file
 * dependency-closure view. Returned by [AnalysisSession.analyze] so
 * callers can ride both onto either response shape — the legacy
 * `buildAnalyze` envelope discards [cacheDeps], the
 * `buildAnalyzeWithDeps` envelope rides it onto the wire.
 */
data class AnalyzeOutcome(
    val result: AnalyzeResult,
    val cacheDeps: OracleResponse.CacheDepsView,
)

data class BatchResult(
    val id: Long,
    val succeeded: Int,
    val skipped: Int,
    val findings: List<Finding>,
    val crashed: Map<String, String>,
    val rules: List<String> = emptyList(),
    // Requested file -> first reason the checker verdict for it is not
    // authoritative (a compiler ERROR, or not part of the JVM compilation).
    val errorFiles: Map<String, String> = emptyMap(),
)

// Holds the current session config. When sourceDirs or classpath change the Go side sends a
// "rebuild" command which disposes this session and creates a new one.
// Analysis runs via K2JVMCompiler with krit-fir registered as a plugin via the fat JAR itself.
class AnalysisSession(val sourceDirs: List<String>, val classpath: List<String>) {

    // Path to the running fat JAR — used to register our FIR plugin with the embedded compiler.
    private val selfJar: String? = resolveSelfJar()

    // Collect all .kt files from sourceDirs. K2 needs all sources for correct type
    // resolution even when checking a subset. Re-walked on every compile because the
    // persistent daemon outlives edits: a file added after startup would otherwise stay
    // invisible to resolution, and a deleted one would linger in freeArgs as a
    // missing-source error. The walk is negligible next to the compile itself.
    private fun currentSourceFiles(): List<String> =
        sourceDirs.flatMap { dir ->
            val canonicalRoot = File(dir).canonicalFile.toPath()
            File(dir).walkTopDown()
                .filter { it.isFile && it.extension == "kt" }
                .map { file ->
                    val relative = canonicalRoot.relativize(file.canonicalFile.toPath())
                    // A symlinked file can escape the root; its walked name still
                    // addresses that file, whereas root + ../... might not.
                    if (relative.startsWith("..")) file.path else File(dir, relative.toString()).path
                }
                .toList()
        }

    // Choose one spelling per physical file. Explicit requests win because Go
    // indexes the response with those exact strings; walked files retain the
    // sourceDirs spelling so unrequested dependencies also match Go's walk.
    private fun compilationFiles(files: List<String>): List<String> {
        val byCanonical = LinkedHashMap<String, String>()
        for (path in currentSourceFiles()) byCanonical.putIfAbsent(File(path).canonicalPath, path)
        for (path in files) byCanonical[File(path).canonicalPath] = path
        return byCanonical.values.toList()
    }

    /**
     * Runs the enabled FIR rule checkers over [files] in one K2 compilation of
     * the whole module: every `.kt` under [sourceDirs] plus the requested files,
     * against [classpath] (plus the bundled stdlib), the same compilation
     * [analyzeFull] runs for oracle facts.
     *
     * The result tells Go where the checker verdict can be trusted:
     *  - `errorFiles` lists requested files the compiler could not analyze
     *    cleanly (an ERROR diagnostic in the file, or a location-less ERROR
     *    that affects the whole compilation), and requested files outside the
     *    JVM compilation (see [excludedFromJvmCompilation]), which are not
     *    compiled at all.
     *  - `crashed` lists every compiled file when the compiler itself crashed.
     */
    fun check(
        id: Long, files: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
    ): BatchResult {
        val (excluded, compiled) = files.partition { excludedFromJvmCompilation(it.path) }
        val errorFiles = linkedMapOf<String, String>()
        for (ref in excluded) errorFiles[ref.path] = NOT_IN_JVM_COMPILATION
        val enabled = FirRuleDiscovery.enabled(FirRuleCompileContext(enabledRules))
        if (compiled.isEmpty()) {
            return BatchResult(
                id = id, succeeded = 0, skipped = excluded.size, findings = emptyList(),
                crashed = emptyMap(), rules = enabled.map { it.ruleId }, errorFiles = errorFiles,
            )
        }
        val requestedPaths = compiled.associateBy { File(it.path).canonicalPath }
            .mapValues { it.value.path }

        val collector = FindingCollector(requestedPaths, enabledRules)
        val outDir = Files.createTempDirectory("krit-fir-out-").toFile()

        FirRuleContext.begin(FirRuleCompileContext(enabledRules, ruleConfigs))
        val exitCode = try {
            val args = K2JVMCompilerArguments().apply {
                freeArgs = compilationFiles(compiled.map { it.path })
                // Go sends the JVM-scoped source roots (non-JVM KMP sets dropped,
                // oracle.FindSourceDirs) and never requests files from dropped
                // roots, and excludedFromJvmCompilation drops any that still
                // arrive, so this compiles common + JVM sources the way the
                // oracle's analyzeFull does.
                MultiplatformSources.configure(this, this@AnalysisSession.sourceDirs, freeArgs)
                this.classpath = effectiveClasspath(this@AnalysisSession.classpath).joinToString(File.pathSeparator)
                destination = outDir.absolutePath
                noStdlib = true
                noReflect = true
                suppressWarnings = false
                // Without this K2 drops every warning, rule findings included,
                // once any file in the module has an error. Go would then read
                // the error-free files as checked and clean.
                reportAllWarnings = true
                if (selfJar != null) {
                    pluginClasspaths = arrayOf(selfJar)
                }
            }
            K2JVMCompiler().exec(collector, Services.EMPTY, args)
        } finally {
            outDir.deleteRecursively()
            FirRuleContext.end()
        }

        val crashMessage = collector.exceptions.firstOrNull()
            ?: if (exitCode == ExitCode.INTERNAL_ERROR) "krit-fir: compiler exited with INTERNAL_ERROR" else null
        val crashed = if (crashMessage != null) compiled.associate { it.path to crashMessage } else emptyMap()
        errorFiles.putAll(collector.errorFiles)
        collector.globalErrors.firstOrNull()?.let { global ->
            for (ref in compiled) errorFiles.putIfAbsent(ref.path, global)
        }
        val gated = crashed.keys + errorFiles.keys
        return BatchResult(
            id = id,
            succeeded = compiled.count { it.path !in gated },
            skipped = excluded.size,
            findings = collector.findings.toList(),
            crashed = crashed,
            rules = enabled.map { it.ruleId },
            errorFiles = errorFiles,
        )
    }

    /**
     * True when [path] sits in a Gradle source-set root (`.../src/<set>/kotlin`
     * or `java`) that is not one of this session's [sourceDirs]. Go only sends
     * the roots of JVM-compilable source sets, so such a file belongs to a
     * dropped target (jsMain, iosMain, ...) and compiling it on the JVM would
     * produce a verdict for code the JVM never compiles. Paths outside that
     * layout, and every path when no source roots were sent, are compiled.
     */
    internal fun excludedFromJvmCompilation(path: String): Boolean {
        if (sourceDirs.isEmpty()) return false
        var dir = File(path).absoluteFile.normalize().parentFile
        while (dir != null) {
            if ((dir.name == "kotlin" || dir.name == "java") && dir.parentFile?.parentFile?.name == "src") {
                return canonicalOrSelf(dir) !in canonicalSourceDirs
            }
            dir = dir.parentFile
        }
        return false
    }

    private val canonicalSourceDirs: Set<String> by lazy { sourceDirs.map { canonicalOrSelf(File(it)) }.toSet() }

    /**
     * Run a K2 frontend compilation to collect oracle-style per-class
     * projections for `files` (or every source in `sourceDirs` when
     * `files` is empty). The result mirrors krit-types' analyze /
     * analyzeAll JSON shape — classes captured during compilation feed
     * through the dispatched [OracleClassChecker] into an
     * [OracleCollector], which the orchestrator drains here.
     *
     * Diagnostic checkers run on the same K2 invocation; warnings from
     * the retained compiler-diagnostic factory subset are projected into each
     * [`FilePayload.diagnostics`] via [`OracleDiagnosticMessageCollector`].
     * Non-matching compiler messages are dropped.
     */
    fun analyze(files: List<String>): AnalyzeResult = analyzeFull(files).result

    /**
     * Same K2 compilation as [analyze] but also drains the
     * collector's [DepTracker] into a [CacheDepsView] so the
     * `analyzeWithDeps` RPC envelope can populate the per-file
     * dependency closure.
     */
    fun analyzeFull(files: List<String>): AnalyzeOutcome {
        val collector = OracleCollector()
        val outDir = Files.createTempDirectory("krit-fir-oracle-out-").toFile()
        OracleCollectorRegistry.begin(collector)
        FirRuleContext.begin(FirRuleCompileContext(noneEnabled = true))
        try {
            val args = K2JVMCompilerArguments().apply {
                freeArgs = compilationFiles(files)
                MultiplatformSources.configure(this, this@AnalysisSession.sourceDirs, freeArgs)
                this.classpath = effectiveClasspath(this@AnalysisSession.classpath).joinToString(File.pathSeparator)
                destination = outDir.absolutePath
                noStdlib = true
                noReflect = true
                // `suppressWarnings = false` + `reportAllWarnings = true`
                // so K2 emits warning-level diagnostics through the message
                // collector even when compilation also finds an error. The
                // plugin's `KritFirCheckers` adds K2's `UnreachableCodeChecker`
                // to its own control-flow checker set so UNREACHABLE_CODE lands
                // here too — the checker lives in the experimental package by
                // default and is not on the standard pipeline. The retained
                // factories are all standard-pipeline warnings, so K2's extended
                // checkers stay off (they would only add USELESS_CALL_ON_NOT_NULL,
                // which no rule consumes yet).
                suppressWarnings = false
                reportAllWarnings = true
                if (selfJar != null) {
                    pluginClasspaths = arrayOf(selfJar)
                }
            }
            val pathByCanonical = args.freeArgs.associateBy { File(it).canonicalPath }
                .mapValues { it.value }
            K2JVMCompiler().exec(OracleDiagnosticMessageCollector(collector, pathByCanonical), Services.EMPTY, args)
        } finally {
            outDir.deleteRecursively()
            OracleCollectorRegistry.end()
            FirRuleContext.end()
        }
        val tracker = collector.depTracker
        return AnalyzeOutcome(
            result = collector.toResult(),
            cacheDeps = OracleResponse.CacheDepsView(
                depPathsByFile = tracker.depPathsByFile,
                perFileDeps = tracker.perFileDeps,
                crashedFiles = tracker.crashedFiles,
            ),
        )
    }

    fun dispose() {} // No long-lived JVM resources.

    companion object {
        internal const val NOT_IN_JVM_COMPILATION =
            "krit-fir: not compiled; the file's source set is not part of the JVM compilation"

        private fun canonicalOrSelf(file: File): String =
            try { file.canonicalPath } catch (_: Exception) { file.absolutePath }

        private fun resolveSelfJar(): String? {
            // Test harnesses override the plugin classpath via this
            // system property — the plain `:jar` task output is enough
            // (the test JVM already has the Kotlin compiler), and
            // pointing here means tests don't need the slower
            // shadow-jar build to register the plugin.
            System.getProperty("krit.fir.plugin.jar")?.let { override ->
                val file = File(override)
                if (file.isFile) return file.absolutePath
            }
            return try {
                val location = AnalysisSession::class.java.protectionDomain?.codeSource?.location
                location?.toURI()?.let { File(it) }?.absolutePath?.takeIf { it.endsWith(".jar") }
            } catch (_: Exception) {
                null
            }
        }
    }
}
