package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.SdkLevels
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.FirRuleErrors
import dev.jasonpearson.krit.fir.isIsolatable
import dev.jasonpearson.krit.fir.oracle.AnalyzeResult
import dev.jasonpearson.krit.fir.oracle.OracleCollector
import dev.jasonpearson.krit.fir.oracle.OracleCollectorRegistry
import dev.jasonpearson.krit.fir.oracle.OracleDiagnosticMessageCollector
import dev.jasonpearson.krit.fir.oracle.OracleResponse
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import org.jetbrains.kotlin.config.JvmTarget
import java.io.File
import java.nio.file.Files

data class FileRef(val path: String, val contentHash: String = "")

internal val prunedSourceDirectoryNames = setOf(
    ".git", ".krit", ".krit-cache", ".krit-types", ".gradle", ".idea", ".kotlin", ".claude", ".codex", ".grit",
)

// Prefer the highest target this embedded compiler knows that the current JDK
// can run. A supported exported Gradle target is authoritative when supplied.
internal fun resolveJvmTarget(declared: String, jdkFeature: Int = Runtime.version().feature()): String {
    if (declared.isNotBlank() && JvmTarget.fromString(declared) != null) {
        return declared
    }
    return JvmTarget.entries
        .filter { target -> target.description.removePrefix("1.").toIntOrNull()?.let { it <= jdkFeature } == true }
        .maxByOrNull { it.description.removePrefix("1.").toInt() }
        ?.description ?: JvmTarget.DEFAULT.description
}

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
    // Rule id -> requested file -> the exception that rule's checker threw
    // there. The compile went on; only that rule's verdict for that file is
    // not authoritative.
    val ruleErrors: Map<String, Map<String, String>> = emptyMap(),
    val modules: List<ModuleStatus> = emptyList(),
    val decidingModules: Map<String, String> = emptyMap(),
    // Includes diagnostics outside the requested subset, for output validity.
    val firstCompilerError: String? = null,
    internal val ownedCompilerError: String? = null,
    internal val compilerCrashed: Boolean = false,
)

// Holds the current session config. When sourceDirs or classpath change the Go side sends a
// "rebuild" command which disposes this session and creates a new one.
// Analysis runs via K2JVMCompiler with krit-fir registered as a plugin via the fat JAR itself.
class AnalysisSession(val sourceDirs: List<String>, val classpath: List<String>, val jvmTarget: String = "") {

    private val compilationJvmTarget: String = resolveJvmTarget(jvmTarget)
    val jvmTargetWarning: String? = if (jvmTarget.isNotBlank() && JvmTarget.fromString(jvmTarget) == null) {
        "Unsupported JVM target '$jvmTarget'; using $compilationJvmTarget"
    } else null

    // Path to the running fat JAR — used to register our FIR plugin with the embedded compiler.
    private val selfJar: String? = resolveSelfJar()

    // Collect all .kt files from sourceDirs. K2 needs all sources for correct type
    // resolution even when checking a subset. Re-walked on every compile because the
    // persistent daemon outlives edits: a file added after startup would otherwise stay
    // invisible to resolution, and a deleted one would linger in freeArgs as a
    // missing-source error. The walk is negligible next to the compile itself.
    internal fun currentSourceFiles(): List<String> =
        sourceDirs.flatMap { dir ->
            val canonicalRoot = File(dir).canonicalFile.toPath()
            File(dir).walkTopDown()
                .onEnter { directory -> directory.name !in prunedSourceDirectoryNames }
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
     * [analyzeFull] runs for oracle facts. [ruleConfigs], [testFiles] (the
     * requested files krit classifies as test files), [scanPaths] (the
     * scan's own spelling of each requested file) and [sdkLevels] (each
     * requested file's resolved minSdk / targetSdk) reach the checkers through
     * [FirRuleContext] with the requested paths; the oracle compile sends
     * none of them.
     *
     * The result tells Go where the checker verdict can be trusted:
     *  - `errorFiles` lists requested files the compiler could not analyze
     *    cleanly (an ERROR diagnostic in the file, or a location-less ERROR
     *    that affects the whole compilation), and requested files that are
     *    not compiled at all: Kotlin scripts and files outside the JVM
     *    compilation (see [excludedFromJvmCompilation]).
     *  - `crashed` lists every compiled file when the compiler itself crashed.
     *  - `ruleErrors` lists, per rule, the requested files on which that
     *    rule's checker threw. The exception is isolated to the rule (see
     *    FirRuleIsolation.kt): the compile and every other rule carry on.
     */
    fun check(
        id: Long, files: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        sdkLevels: Map<String, SdkLevels> = emptyMap(),
    ): BatchResult {
        return checkCompilation(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels)
    }

    internal fun checkCompilation(
        id: Long, files: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>>,
        testFiles: Set<String>, scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
        module: ModuleCompilation? = null,
        ownedSources: Set<String> = emptySet(),
    ): BatchResult {
        val (excluded, compiled) = files.partition {
            isScript(it.path) || (module == null && excludedFromJvmCompilation(it.path))
        }
        val errorFiles = linkedMapOf<String, String>()
        for (ref in excluded) errorFiles[ref.path] = if (isScript(ref.path)) SCRIPT_NOT_COMPILED else NOT_IN_JVM_COMPILATION
        val enabled = FirRuleDiscovery.enabled(FirRuleCompileContext(enabledRules))
        if (compiled.isEmpty() && module == null) {
            return BatchResult(
                id = id, succeeded = 0, skipped = excluded.size, findings = emptyList(),
                crashed = emptyMap(), rules = enabled.map { it.ruleId }, errorFiles = errorFiles,
            )
        }
        val requestedPaths = compiled.associateBy { File(it.path).canonicalPath }
            .mapValues { it.value.path }

        val collector = FindingCollector(requestedPaths, enabledRules, ownedSources)
        val ruleErrorRecorder = FirRuleErrorRecorder(if (module == null) null else compiled.mapTo(HashSet()) { it.path })
        val outDir = module?.output ?: Files.createTempDirectory("krit-fir-out-").toFile()

        val ruleContext = FirRuleCompileContext(
            enabledRules, ruleConfigs, testFiles = testFiles,
            files = compiled.mapTo(LinkedHashSet()) { it.path }, scanPaths = scanPaths,
            sdkLevels = sdkLevels,
        )
        val exitCode = try {
            val args = compilationArguments(module?.sources ?: compilationFiles(compiled.map { it.path }), outDir, module)
            compileModule(args, listOf(collector), listOf(
                CompilationContext({ FirRuleContext.begin(ruleContext) }, { FirRuleContext.end() }),
                CompilationContext({ FirRuleErrors.begin(ruleErrorRecorder) }, { FirRuleErrors.end() }),
            ), skipEmptySources = true)
        } catch (e: Exception) {
            if (module == null || !isIsolatable(e)) throw e
            collector.exceptions += (e.message ?: e.javaClass.name)
            ExitCode.INTERNAL_ERROR
        } finally {
            if (module == null) outDir.deleteRecursively()
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
            ruleErrors = requestedRuleErrors(ruleErrorRecorder.snapshot(), requestedPaths, compiled),
            ownedCompilerError = crashMessage ?: collector.globalErrors.firstOrNull() ?: collector.ownedError,
            compilerCrashed = crashMessage != null,
            firstCompilerError = crashMessage ?: collector.firstError
                ?: if (exitCode != ExitCode.OK) "Compiler exited with $exitCode" else null,
        )
    }

    // Maps recorded rule errors onto request spellings. Errors in files that
    // were compiled but not requested are dropped (Go never reads FIR's verdict
    // for them); an error with no file applies to every compiled request.
    private fun requestedRuleErrors(
        recorded: Map<String, Map<String, String>>,
        requestedPaths: Map<String, String>,
        compiled: List<FileRef>,
    ): Map<String, Map<String, String>> {
        val out = linkedMapOf<String, MutableMap<String, String>>()
        for ((ruleId, byPath) in recorded) {
            for ((path, message) in byPath) {
                val targets = if (path.isEmpty()) {
                    compiled.map { it.path }
                } else {
                    listOfNotNull(requestedPaths[canonicalOrSelf(File(path))])
                }
                for (target in targets) out.getOrPut(ruleId) { linkedMapOf() }.putIfAbsent(target, message)
            }
        }
        return out
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

    // Kotlin scripts (build.gradle.kts, ...) compile against script
    // definitions the module compilation does not have; like the oracle,
    // which only compiles `.kt`, the check never compiles them.
    private fun isScript(path: String): Boolean = !path.endsWith(".kt")

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
        try {
            val args = compilationArguments(compilationFiles(files), outDir)
            val pathByCanonical = args.freeArgs.associateBy { File(it).canonicalPath }
                .mapValues { it.value }
            compileModule(args, listOf(OracleDiagnosticMessageCollector(collector, pathByCanonical)), listOf(
                CompilationContext({ OracleCollectorRegistry.begin(collector) }, { OracleCollectorRegistry.end() }),
                CompilationContext({ FirRuleContext.begin(FirRuleCompileContext(noneEnabled = true)) }, { FirRuleContext.end() }),
            ))
        } finally {
            outDir.deleteRecursively()
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

    /** Shared defaults for legacy oracle/checker and module compiles. Output ownership stays with the caller. */
    private fun compilationArguments(
        sources: List<String>, output: File, module: ModuleCompilation? = null,
    ): K2JVMCompilerArguments = K2JVMCompilerArguments().apply {
        freeArgs = sources
        if (module == null) MultiplatformSources.configure(this, sourceDirs, freeArgs)
        classpath = effectiveClasspath(this@AnalysisSession.classpath).joinToString(File.pathSeparator)
        jvmTarget = compilationJvmTarget
        destination = output.absolutePath
        noStdlib = true
        noReflect = true
        // Keep warning diagnostics (including rule findings) even when another file has errors.
        // Extended compiler checkers remain off, preserving the oracle's diagnostic subset.
        suppressWarnings = false
        reportAllWarnings = true
        if (selfJar != null) pluginClasspaths = arrayOf(selfJar)
        module?.configure(this, compilationJvmTarget)
    }

    /** A caller-supplied registry scope; the execution seam knows nothing about its payload. */
    private class CompilationContext(val begin: () -> Unit, val end: () -> Unit)

    /**
     * The single embedded-compiler execution seam. Arguments carry destination and backend
     * options; this function never clears/deletes outputs. Callers can supply several message
     * collectors and registry contexts together without changing the execution lifecycle.
     * A future frontend-only backend can be selected here without changing either caller.
     */
    private fun compileModule(
        args: K2JVMCompilerArguments,
        collectors: List<MessageCollector>,
        contexts: List<CompilationContext>,
        skipEmptySources: Boolean = false,
    ): ExitCode {
        val messages = object : MessageCollector {
            override fun clear() = collectors.forEach { it.clear() }
            override fun hasErrors() = collectors.any { it.hasErrors() }
            override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) =
                collectors.forEach { it.report(severity, message, location) }
        }
        fun execute(index: Int): ExitCode {
            if (index == contexts.size) {
                // Module/check requests with no sources must not enter the compiler REPL.
                return if (skipEmptySources && args.freeArgs.isEmpty()) ExitCode.OK
                else K2JVMCompiler().exec(messages, Services.EMPTY, args)
            }
            val context = contexts[index]
            context.begin()
            return try { execute(index + 1) } finally { context.end() }
        }
        return execute(0)
    }

    private var retainedModuleRunner: ModuleRunner? = null
    internal val moduleRunner: ModuleRunner get() = retainedModuleRunner ?: ModuleRunner().also { retainedModuleRunner = it }

    internal fun rebuild(sourceDirs: List<String>, classpath: List<String>, jvmTarget: String): AnalysisSession =
        AnalysisSession(sourceDirs, classpath, jvmTarget).also {
            it.retainedModuleRunner = retainedModuleRunner
            retainedModuleRunner = null
        }

    /**
     * Takes back the module state [rebuilt] received from [rebuild] when the
     * request that triggered the rebuild failed and this session stays active.
     */
    internal fun reclaimRetainedState(rebuilt: AnalysisSession) {
        retainedModuleRunner = rebuilt.retainedModuleRunner
        rebuilt.retainedModuleRunner = null
    }

    fun analyzeModules(
        id: Long, modules: List<ModuleSpec>, checkFiles: List<String>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(), scanPaths: Map<String, String> = emptyMap(),
        sdkLevels: Map<String, SdkLevels> = emptyMap(),
    ): BatchResult = if (modules.isEmpty()) {
        check(id, checkFiles.map { FileRef(it) }, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels)
    } else {
        moduleRunner.check(id, modules, checkFiles, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels,
            compilationJvmTarget)
    }

    internal val moduleCompilationCounts: Map<String, Int> get() = moduleRunner.compilationCounts

    internal val moduleOutputDirectories: Map<String, File> get() = moduleRunner.outputDirectories

    fun dispose() { retainedModuleRunner?.dispose(); retainedModuleRunner = null }

    companion object {
        internal const val SCRIPT_NOT_COMPILED =
            "krit-fir: not compiled; Kotlin scripts are not part of the module compilation"
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
