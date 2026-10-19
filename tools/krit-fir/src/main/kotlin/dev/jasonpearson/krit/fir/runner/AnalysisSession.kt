package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.SdkLevels
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.FirRuleErrors
import dev.jasonpearson.krit.fir.incremental.CompileRecording
import dev.jasonpearson.krit.fir.incremental.CompileRecordingRegistry
import dev.jasonpearson.krit.fir.incremental.FingerprintBuilder
import dev.jasonpearson.krit.fir.incremental.IncrementalStage
import dev.jasonpearson.krit.fir.incremental.IncrementalState
import dev.jasonpearson.krit.fir.incremental.IncrementalTestHook
import dev.jasonpearson.krit.fir.incremental.SNAPSHOT_MODULE_NAME
import dev.jasonpearson.krit.fir.incremental.SliceFallback
import dev.jasonpearson.krit.fir.incremental.SliceOutcome
import dev.jasonpearson.krit.fir.incremental.Snapshot
import dev.jasonpearson.krit.fir.incremental.SnapshotBuild
import dev.jasonpearson.krit.fir.incremental.SnapshotComponents
import dev.jasonpearson.krit.fir.incremental.SnapshotStorage
import dev.jasonpearson.krit.fir.incremental.contentHashesOf
import dev.jasonpearson.krit.fir.incremental.incrementalRecordingFromEnv
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
import org.jetbrains.kotlin.config.KotlinCompilerVersion
import org.jetbrains.kotlin.incremental.components.LookupTracker
import org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCompilationComponents
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
    // Compiler settings or arguments that were requested but not applied (unsupported
    // flag, krit-managed argument, version the embedded compiler lacks). Never silent.
    val warnings: List<String> = emptyList(),
)

// Holds the current session config. When sourceDirs or classpath change the Go side sends a
// "rebuild" command which disposes this session and creates a new one.
// Analysis runs via K2JVMCompiler with krit-fir registered as a plugin via the fat JAR itself.
class AnalysisSession(
    val sourceDirs: List<String>,
    val classpath: List<String>,
    val jvmTarget: String = "",
    /** The project's Kotlin compiler settings as compiler arguments; empty keeps the embedded compiler's defaults. */
    val kotlinArgs: List<String> = emptyList(),
    /**
     * Records what incremental checking needs from each whole-module check compile: the compiler's lookups
     * and every file's declaration surface, kept in [incrementalState]. Off keeps the compile exactly as it
     * is without the feature. Defaults to `KRIT_FIR_INCREMENTAL=1`.
     */
    val incrementalRecording: Boolean = incrementalRecordingFromEnv(),
    /**
     * After a clean whole-module check compile with [incrementalRecording] on, builds the module's binary snapshot
     * in the same compile (the backend runs after the frontend that produced the verdicts), for slice compiles
     * ([sliceCheck]). A failure to build it loses the snapshot only; the check's result is the same either way.
     */
    val incrementalSnapshot: Boolean = incrementalRecording,
) {

    /** Parent of the snapshot directories, for tests; null uses the per-user krit cache dir. */
    internal var snapshotRootOverride: File? = null

    /** Lets tests adjust the compiler arguments of every non-module compile (Java source roots). */
    internal var argumentsHook: ((K2JVMCompilerArguments) -> Unit)? = null

    // Serializes whatever touches the snapshot directory: slice compiles (which move files in it), and the commits
    // of a whole-module compile (which replace and delete it).
    private val snapshotLock = Any()

    /** Per-file surface hashes, lookup reverse index and content hashes of this session's last recorded check compile. */
    internal var incrementalState = IncrementalState()
        internal set

    @Volatile private var recordingFailureLogged = false
    @Volatile private var snapshotFailureLogged = false

    private fun snapshotFailed(cause: Throwable) {
        if (!snapshotFailureLogged) {
            snapshotFailureLogged = true
            System.err.println("krit-fir: incremental snapshot failed and was dropped (${cause.javaClass.name}: ${cause.message})")
        }
    }

    // Recording is best-effort: whatever goes wrong with it drops the recorded facts and never touches the verdict.
    private fun recordingFailed(cause: Throwable) {
        try { synchronized(snapshotLock) { incrementalState.reset() } } catch (_: Throwable) {}
        if (!recordingFailureLogged) {
            recordingFailureLogged = true
            System.err.println("krit-fir: incremental recording failed and was dropped (${cause.javaClass.name}: ${cause.message})")
        }
    }

    private val compilationJvmTarget: String = resolveJvmTarget(jvmTarget)
    val jvmTargetWarning: String? = if (jvmTarget.isNotBlank() && JvmTarget.fromString(jvmTarget) == null) {
        "Unsupported JVM target '$jvmTarget'; using $compilationJvmTarget"
    } else null

    /** The allowlist-filtered settings applied to every non-module compilation of this session. */
    val kotlinSettings: KotlinCompilerSettings = KotlinCompilerSettings.parse(kotlinArgs)

    /** Why part of [kotlinArgs] was not applied; surfaced in the response so it is never silent. */
    val kotlinSettingsWarnings: List<String> = kotlinSettings.warnings.map { "krit-fir: $it" }

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
    private fun compilationFiles(files: List<String>, walked: List<String> = currentSourceFiles()): List<String> {
        val byCanonical = LinkedHashMap<String, String>()
        for (path in walked) byCanonical.putIfAbsent(File(path).canonicalPath, path)
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
        val plan = CheckPlan(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, module, ownedSources)
        if (plan.compiled.isEmpty() && module == null) return plan.emptyResult()
        val outDir = module?.output ?: Files.createTempDirectory("krit-fir-out-").toFile()
        val exitCode = try {
            val sources = module?.sources ?: compilationFiles(plan.compiled.map { it.path })
            val args = compilationArguments(sources, outDir, module)
            // A session runs many module compiles; only the legacy whole-session compile is recorded.
            if (module == null) plan.startRecording(sources)
            val build = if (module == null) plan.prepareSnapshot(args, sources) else null
            compileModule(args, listOf(plan.collector), plan.contexts(), skipEmptySources = true, frontendOnly = module == null,
                recording = plan.recording, build = build).also { plan.commitRecording(it, build) }
        } catch (e: Exception) {
            if (module == null || !isIsolatable(e)) throw e
            plan.collector.exceptions += (e.message ?: e.javaClass.name)
            ExitCode.INTERNAL_ERROR
        } finally {
            plan.discardSnapshot()
            if (module == null) outDir.deleteRecursively()
        }
        return plan.result(exitCode)
    }

    /**
     * True when the files kept in every slice (binary-stale or deleted since the snapshot) passed the bound: the
     * planner then serves the request with a full [check], which builds a fresh snapshot and replaces the state.
     */
    internal fun needsResnapshot(
        maxFraction: Double = IncrementalState.DEFAULT_STALE_FRACTION,
        minFiles: Int = IncrementalState.DEFAULT_STALE_MIN_FILES,
    ): Boolean = incrementalState.exceedsStaleBound(maxFraction, minFiles)

    /**
     * Why [sliceCheck] cannot be used right now, or null when it can: no snapshot (never built, the module has a
     * compiler error, building failed, or a slice damaged it), the module uses `expect`/`actual`, or the compiler
     * facts changed since the snapshot was built. The planner serves the request with a full check in each case.
     */
    internal fun sliceFallback(): SliceFallback? {
        incrementalState.fallbackReason()?.let { return it }
        return if (incrementalState.compileKey != compileKey()) SliceFallback.STATE_CHANGED else null
    }

    /**
     * Checks [dirtyFiles] against the module's binary snapshot: the files are compiled from source through the
     * stock frontend with the snapshot as the module's own previous output, and every other file of the module
     * comes from its binary. Constructs that cross the source/binary line (`internal`, facades and multifile
     * classes, inline functions, constants, sealed parents with sources on both sides) resolve as in a full
     * compile, with the exceptions listed below.
     *
     * The set that is compiled is [dirtyFiles] plus whatever a slice needs to be sound, which `sliceCheck`
     * enforces itself and reports in [SliceOutcome.Checked.compiled]:
     *  - [IncrementalState.binaryStale]: a file whose binary no longer describes it must not be read from the
     *    snapshot;
     *  - the files of every subclass of a sealed class or interface declared in a compiled file, over the whole
     *    sealed hierarchy. A sealed parent compiled from source knows only the inheritors that are in source, so
     *    without them its inheritor list, and with it its surface and every exhaustiveness check, would be wrong.
     *    The set is re-checked against the compiled surfaces and the slice is compiled again when it grew.
     *
     * The result is today's per-file shape for the compiled files: findings, errorFiles and ruleErrors. The slice's
     * lookups and surfaces are merged into [incrementalState]: the compiled files' replace theirs, everything
     * else stays. A dirty file that no longer exists is a deletion, like those in [removedFiles]; the snapshot's
     * binaries of deleted files stay hidden from every slice. Returns [SliceOutcome.Fallback] instead of compiling
     * when [sliceFallback] says so, and when the slice compile throws or crashes (the snapshot and the state are
     * then untouched).
     *
     * What remains with the caller (the planner):
     *  - the dependents of what changed (lookups, plus the structural rules of the findings), and, for a new
     *    subclass of a sealed parent in another file, that parent's file (this method adds a sealed parent's
     *    subclasses, not a subclass's parent);
     *  - a redeclaration or conflicting overload is only seen when both sides are in the slice;
     *  - verdicts of the whole-compilation tiered rules (`FirRule.wholeCompilationTier`) are not those of a full
     *    compile on a slice and must not be trusted; step 4 decides how they are served;
     *  - a module that uses `expect`/`actual` has no snapshot ([SliceFallback.EXPECT_ACTUAL]).
     */
    internal fun sliceCheck(
        id: Long, dirtyFiles: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        sdkLevels: Map<String, SdkLevels> = emptyMap(),
        removedFiles: Collection<String> = emptyList(),
    ): SliceOutcome = synchronized(snapshotLock) {
        sliceFallback()?.let { return SliceOutcome.Fallback(it) }
        val snapshot = incrementalState.snapshot ?: return SliceOutcome.Fallback(SliceFallback.NO_SNAPSHOT)
        val refs = LinkedHashMap<String, FileRef>()
        for (ref in dirtyFiles) refs.putIfAbsent(canonicalOrSelf(File(ref.path)), ref)
        val requested = refs.keys.toSet()
        for (stale in incrementalState.binaryStale()) refs.putIfAbsent(stale, FileRef(stale))
        var wanted = incrementalState.withSealedInheritors(refs.keys)
        while (true) {
            val (present, missing) = wanted.partition { File(it).isFile }
            val removed = (removedFiles + missing).mapTo(LinkedHashSet()) { canonicalOrSelf(File(it)) }
            val plan = CheckPlan(
                id, present.map { refs[it] ?: FileRef(it) }, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, null, emptySet(),
            )
            val sources = plan.compiled.map { it.path }
            plan.startRecording(sources)
            val recording = plan.recording ?: return SliceOutcome.Fallback(SliceFallback.NO_SNAPSHOT)
            val compiled = sources.mapTo(LinkedHashSet()) { canonicalOrSelf(File(it)) }
            if (sources.isEmpty()) {
                // Only deletions (or files krit never compiles): nothing to compile.
                if (removed.isNotEmpty()) mergeSlice(plan, compiled, removed, emptySet())
                return SliceOutcome.Checked(plan.emptyResult(), compiled, compiled - requested)
            }
            val outDir = Files.createTempDirectory("krit-fir-slice-out-").toFile()
            try {
                val args = compilationArguments(sources, outDir)
                args.incrementalCompilation = true
                args.moduleName = SNAPSHOT_MODULE_NAME
                args.classpath = listOfNotNull(args.classpath, snapshot.dir.path).joinToString(File.pathSeparator)
                val hidden = sources + removed + incrementalState.removedSinceSnapshot()
                val exit = try {
                    snapshot.parked(hidden) {
                        compileModule(args, listOf(plan.collector), plan.contexts(), frontendOnly = true, recording = recording, slice = snapshot)
                    }
                } catch (e: Throwable) {
                    if (!isIsolatable(e)) throw e
                    return sliceFailed(e, snapshot)
                }
                if (exit == ExitCode.INTERNAL_ERROR || plan.collector.exceptions.isNotEmpty()) {
                    return sliceFailed(IllegalStateException(plan.collector.exceptions.firstOrNull() ?: "slice compile exited with $exit"), snapshot)
                }
                // A sealed class declared in a compiled file (it may have become sealed in this very edit) needs
                // all its inheritors in source.
                val grown = incrementalState.withSealedInheritors(compiled, recording.surfaces)
                if ((grown - compiled - removed).any { File(it).isFile }) {
                    wanted = grown + wanted
                    continue
                }
                val result = plan.result(exit)
                val errored = result.errorFiles.keys.mapTo(HashSet()) { canonicalOrSelf(File(it)) }
                mergeSlice(plan, compiled, removed, errored)
                return SliceOutcome.Checked(result, compiled, compiled - requested)
            } finally {
                outDir.deleteRecursively()
            }
        }
        @Suppress("UNREACHABLE_CODE") error("unreachable")
    }

    private fun sliceFailed(cause: Throwable, snapshot: Snapshot): SliceOutcome {
        incrementalState.noteSliceFailure(cause)
        System.err.println("krit-fir: slice compile failed (${cause.javaClass.name}: ${cause.message}); falling back to a full compile")
        // A snapshot whose files could not be put back is unusable.
        if (snapshot.corrupt) incrementalState.dropSnapshot(SliceFallback.SLICE_FAILED)
        return SliceOutcome.Fallback(SliceFallback.SLICE_FAILED, cause)
    }

    // Best-effort like all recording: if the facts cannot be merged they are dropped (and with them the snapshot),
    // and the verdict, which is already computed, is returned unchanged.
    private fun mergeSlice(plan: CheckPlan, compiled: Set<String>, removed: Set<String>, errored: Set<String>) {
        try {
            val recorded = plan.recording
            if (recorded == null || recorded.failed) {
                recordingFailed(recorded?.failure ?: IllegalStateException("recording failed"))
            } else {
                incrementalState.commitSlice(recorded, compiled, removed, errored)
            }
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            recordingFailed(e)
        }
    }

    /**
     * One check request's setup and verdict: which requested files are
     * compiled, the collectors and registry contexts the checkers report
     * through, and the [BatchResult] built from them after the compile.
     * Shared by [checkCompilation] and [analyzeWithCheck], so the merged
     * compile reaches the same verdict as a standalone check.
     */
    private inner class CheckPlan(
        val id: Long, files: List<FileRef>, val enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>>,
        testFiles: Set<String>, scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
        module: ModuleCompilation?,
        ownedSources: Set<String>,
    ) {
        val excluded: List<FileRef>
        val compiled: List<FileRef>
        init {
            val (out, inside) = files.partition {
                isScript(it.path) || (module == null && excludedFromJvmCompilation(it.path))
            }
            excluded = out
            compiled = inside
        }
        private val errorFiles = linkedMapOf<String, String>().apply {
            for (ref in excluded) put(ref.path, if (isScript(ref.path)) SCRIPT_NOT_COMPILED else NOT_IN_JVM_COMPILATION)
        }
        private val enabled = FirRuleDiscovery.enabled(FirRuleCompileContext(enabledRules))
        private val requestedPaths = compiled.associateBy { File(it.path).canonicalPath }.mapValues { it.value.path }
        val collector = FindingCollector(requestedPaths, enabledRules, ownedSources)
        private val ruleErrorRecorder = FirRuleErrorRecorder(if (module == null) null else compiled.mapTo(HashSet()) { it.path })
        private val ruleContext = FirRuleCompileContext(
            enabledRules, ruleConfigs, testFiles = testFiles,
            files = compiled.mapTo(LinkedHashSet()) { it.path }, scanPaths = scanPaths,
            sdkLevels = sdkLevels,
        )

        // Incremental recording of this compile; null unless the session records.
        var recording: CompileRecording? = null
            private set
        private val requestVerdictKey: String by lazy { verdictKey(enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels) }

        // The snapshot this plan's compile is writing, until the state takes it over.
        private var pendingSnapshot: Snapshot? = null
        private var noSnapshotReason = SliceFallback.NO_SNAPSHOT

        /**
         * Sets up the snapshot build for a whole-module compile that is recording: the compile then also runs the
         * backend into a snapshot directory under krit's cache dir. Null (the compile stays frontend-only) when
         * snapshots are off, the module uses expect/actual, or the directory cannot be made.
         */
        fun prepareSnapshot(args: K2JVMCompilerArguments, sources: List<String>): SnapshotBuild? {
            val recorded = recording ?: return null
            if (!incrementalSnapshot) return null
            try {
                if (MultiplatformSources.usesExpectActual(args, sources)) {
                    noSnapshotReason = SliceFallback.EXPECT_ACTUAL
                    return null
                }
                val snapshot = Snapshot(SnapshotStorage.newSnapshotRoot(snapshotRootOverride))
                pendingSnapshot = snapshot
                snapshot.dir.mkdirs()
                args.destination = snapshot.dir.path
                args.reportOutputFiles = true
                args.moduleName = SNAPSHOT_MODULE_NAME
                return SnapshotBuild(snapshot, recorded.lookups)
            } catch (e: Throwable) {
                if (!isIsolatable(e)) throw e
                snapshotFailed(e)
                discardSnapshot()
                return null
            }
        }

        fun discardSnapshot() {
            val pending = pendingSnapshot ?: return
            pendingSnapshot = null
            try { pending.delete() } catch (_: Exception) {}
        }

        fun startRecording(sources: List<String>) {
            if (!incrementalRecording) return
            try {
                recording = CompileRecording(compileKey(), requestVerdictKey, contentHashesOf(sources))
            } catch (e: Throwable) {
                if (!isIsolatable(e)) throw e
                recording = null
                recordingFailed(e)
            }
        }

        /**
         * Keeps the recording unless the compiler crashed or the recording failed, when it would be partial. The
         * snapshot [build] wrote goes to the state with it, when the compile was clean and the backend succeeded.
         */
        fun commitRecording(exitCode: ExitCode, build: SnapshotBuild? = null) {
            val recorded = recording ?: return
            try {
                if (recorded.failed) { recordingFailed(recorded.failure ?: IllegalStateException("recording failed")); return }
                if (exitCode != ExitCode.INTERNAL_ERROR && collector.exceptions.isEmpty()) {
                    build?.failure?.let { snapshotFailed(it) }
                    synchronized(snapshotLock) {
                        incrementalState.commit(recorded)
                        val snapshot = build?.takeIf { it.built }?.snapshot
                        incrementalState.attachSnapshot(snapshot, noSnapshotReason)
                        if (snapshot != null) pendingSnapshot = null
                    }
                }
            } catch (e: Throwable) {
                if (!isIsolatable(e)) throw e
                recordingFailed(e)
            }
        }

        fun contexts(): List<CompilationContext> = listOfNotNull(
            CompilationContext({ FirRuleContext.begin(ruleContext) }, { FirRuleContext.end() }),
            CompilationContext({ FirRuleErrors.begin(ruleErrorRecorder) }, { FirRuleErrors.end() }),
            recording?.let { r -> CompilationContext({ CompileRecordingRegistry.begin(r) }, { CompileRecordingRegistry.end() }) },
        )

        fun emptyResult(): BatchResult = BatchResult(
            id = id, succeeded = 0, skipped = excluded.size, findings = emptyList(),
            crashed = emptyMap(), rules = enabled.map { it.ruleId }, errorFiles = errorFiles,
        )

        fun result(exitCode: ExitCode): BatchResult {
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
    fun analyzeFull(files: List<String>): AnalyzeOutcome = analyzeCompilation(compilationFiles(files), null)

    /**
     * [analyzeFull] for [files] and [check] for [checkRequest] from one K2
     * compilation (#739): the oracle collector and the enabled rule checkers
     * are registered on the same frontend run instead of each compiling the
     * module. The rule context never reaches the oracle checkers, and the
     * oracle message collector drops krit rule diagnostics, so each side sees
     * what its own compile would have given it.
     *
     * The compile is shared only when both requests compile the same sources:
     * the check's compiled files must already be part of the oracle's
     * compilation and vice versa (compared canonically; the oracle's spelling
     * is kept, and the check maps every path back to its request spelling),
     * and the request's compile context, when it names one, must be this
     * session's. Otherwise, or when the shared compile throws, the check
     * result is null and the oracle compiles alone, so the caller checks on
     * its own.
     */
    fun analyzeWithCheck(files: List<String>, checkRequest: CheckRequest): Pair<AnalyzeOutcome, BatchResult?> {
        val walked = currentSourceFiles()
        val oracleSources = compilationFiles(files, walked)
        val plan = CheckPlan(
            checkRequest.id, checkRequest.files, checkRequest.rules.toSet(), checkRequest.ruleConfigs,
            checkRequest.testFiles, checkRequest.scanPaths, checkRequest.sdkLevels, module = null, ownedSources = emptySet(),
        )
        val shared = plan.compiled.isNotEmpty() &&
            sharesCompileContext(checkRequest) &&
            canonicalSet(oracleSources) == canonicalSet(compilationFiles(plan.compiled.map { it.path }, walked))
        if (!shared) return analyzeFull(files) to null
        var exitCode = ExitCode.OK
        val outcome = try {
            plan.startRecording(oracleSources)
            analyzeCompilation(oracleSources, plan) { exitCode = it; plan.commitRecording(it) }
        } catch (e: Exception) {
            // A checker failure the rule isolation does not absorb must not
            // cost the oracle its facts: compile for the oracle alone, and
            // let the caller's own check meet the failure as before.
            System.err.println("krit-fir: shared oracle/check compile failed (${e.message}); compiling the oracle alone")
            return analyzeFull(files) to null
        }
        return outcome to plan.result(exitCode)
    }

    private fun sharesCompileContext(request: CheckRequest): Boolean {
        fun sameFiles(requested: List<String>, own: List<String>) =
            requested.isEmpty() || requested.map { canonicalOrSelf(File(it)) } == own.map { canonicalOrSelf(File(it)) }
        return sameFiles(request.sourceDirs, sourceDirs) && sameFiles(request.classpath, classpath) &&
            (request.jvmTarget.isEmpty() || request.jvmTarget == jvmTarget) &&
            (request.kotlinArgs.isEmpty() || KotlinCompilerSettings.parse(request.kotlinArgs).identity == kotlinSettings.identity)
    }

    private fun canonicalSet(paths: List<String>): Set<String> = paths.mapTo(HashSet()) { canonicalOrSelf(File(it)) }

    private fun analyzeCompilation(
        sources: List<String>, check: CheckPlan?, onExit: (ExitCode) -> Unit = {},
    ): AnalyzeOutcome {
        val collector = OracleCollector()
        val outDir = Files.createTempDirectory("krit-fir-oracle-out-").toFile()
        try {
            val args = compilationArguments(sources, outDir)
            val pathByCanonical = args.freeArgs.associateBy { File(it).canonicalPath }
                .mapValues { it.value }
            val oracleMessages = OracleDiagnosticMessageCollector(collector, pathByCanonical)
            val oracleContext = CompilationContext({ OracleCollectorRegistry.begin(collector) }, { OracleCollectorRegistry.end() })
            onExit(if (check == null) {
                compileModule(args, listOf(oracleMessages), listOf(
                    oracleContext,
                    CompilationContext({ FirRuleContext.begin(FirRuleCompileContext(noneEnabled = true)) }, { FirRuleContext.end() }),
                ), frontendOnly = true)
            } else {
                compileModule(args, listOf(oracleMessages, check.collector), listOf(oracleContext) + check.contexts(), frontendOnly = true,
                    recording = check.recording)
            })
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
        // Session-level settings describe the whole-repo compilation. A module
        // compilation takes only its own module's arguments (ModuleCompilation),
        // so one module's settings are never applied to another.
        if (module == null) kotlinSettings.applyTo(this)
        destination = output.absolutePath
        noStdlib = true
        noReflect = true
        // Keep warning diagnostics (including rule findings) even when another file has errors.
        // Extended compiler checkers remain off, preserving the oracle's diagnostic subset.
        suppressWarnings = false
        reportAllWarnings = true
        if (selfJar != null) pluginClasspaths = arrayOf(selfJar)
        module?.configure(this, compilationJvmTarget)
        if (module == null) argumentsHook?.invoke(this)
    }

    /**
     * Key of the compiler facts (lookups, surfaces): the compiler and plugin builds, this session's sources,
     * classpath (with file identity), JVM target and Kotlin settings. A different key resets [incrementalState].
     */
    private fun compileKey(): String {
        val builder = FingerprintBuilder()
            .add("kotlin", KotlinCompilerVersion.VERSION)
            .add("sourceDirs", sourceDirs.map { canonicalOrSelf(File(it)) })
            .add("jvmTarget", compilationJvmTarget)
            .add("kotlinSettings", kotlinSettings.identity)
        selfJar?.let { builder.addFileIdentity("plugin", it) }
        for (entry in effectiveClasspath(classpath)) builder.addFileIdentity("classpath", entry)
        return builder.build()
    }

    /** Key of what a verdict depends on beyond the compiler facts: rules, configs, test files, scan paths, SDK levels. */
    private fun verdictKey(
        enabledRules: Set<String>, ruleConfigs: Map<String, Map<String, Any?>>,
        testFiles: Set<String>, scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
    ): String = FingerprintBuilder()
        .add("rules", enabledRules)
        .add("ruleConfigs", ruleConfigs)
        .add("testFiles", testFiles)
        .add("scanPaths", scanPaths)
        .add("sdkLevels", sdkLevels)
        .build()

    /** A caller-supplied registry scope; the execution seam knows nothing about its payload. */
    private class CompilationContext(val begin: () -> Unit, val end: () -> Unit)

    /**
     * The single embedded-compiler execution seam. Arguments carry destination and backend
     * options; this function never clears/deletes outputs. Callers can supply several message
     * collectors and registry contexts together without changing the execution lifecycle.
     *
     * [frontendOnly] stops after the K2 frontend. FIR checkers (krit's and the
     * compiler's) and all diagnostics run inside the frontend, so callers that
     * only read findings or oracle facts skip fir2ir and the JVM backend, about
     * 45% of a clean project's compile (#737). Module compiles keep the backend:
     * downstream modules compile against their class output.
     */
    private fun compileModule(
        args: K2JVMCompilerArguments,
        collectors: List<MessageCollector>,
        contexts: List<CompilationContext>,
        skipEmptySources: Boolean = false,
        frontendOnly: Boolean = false,
        recording: CompileRecording? = null,
        slice: Snapshot? = null,
        build: SnapshotBuild? = null,
    ): ExitCode {
        // The compiler's own incremental-compilation services, wired the way Gradle/JPS do: dependency recording,
        // and for a slice compile the module's previous output. Without either the services stay empty, so the
        // compile is the one that runs without the feature.
        val services = if (recording == null && slice == null) Services.EMPTY else {
            Services.Builder().apply {
                recording?.let { register(LookupTracker::class.java, it.lookups) }
                slice?.let { register(IncrementalCompilationComponents::class.java, SnapshotComponents(it)) }
            }.build()
        }
        val messages = object : MessageCollector {
            override fun clear() = collectors.forEach { it.clear() }
            override fun hasErrors() = collectors.any { it.hasErrors() }
            override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) =
                collectors.forEach { it.report(severity, message, location) }
        }
        fun execute(index: Int): ExitCode {
            if (index == contexts.size) {
                // Module/check requests with no sources must not enter the compiler REPL.
                if (slice != null) IncrementalTestHook.onStage?.invoke(IncrementalStage.SLICE_COMPILE)
                return when {
                    skipEmptySources && args.freeArgs.isEmpty() -> ExitCode.OK
                    build != null -> build.run(args, services, messages)
                    frontendOnly -> FrontendOnlyJvmPipeline().execute(args, services, messages)
                    else -> K2JVMCompiler().exec(messages, services, args)
                }
            }
            val context = contexts[index]
            context.begin()
            return try { execute(index + 1) } finally { context.end() }
        }
        return execute(0)
    }

    private var retainedModuleRunner: ModuleRunner? = null
    internal val moduleRunner: ModuleRunner get() = retainedModuleRunner ?: ModuleRunner().also { retainedModuleRunner = it }

    internal fun rebuild(
        sourceDirs: List<String>, classpath: List<String>, jvmTarget: String, kotlinArgs: List<String> = this.kotlinArgs,
    ): AnalysisSession =
        AnalysisSession(sourceDirs, classpath, jvmTarget, kotlinArgs, incrementalRecording, incrementalSnapshot).also {
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

    fun dispose() {
        retainedModuleRunner?.dispose(); retainedModuleRunner = null
        // Also deletes the snapshot directory.
        synchronized(snapshotLock) { incrementalState.reset() }
    }

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
