package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.ClassNameFact
import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.SdkLevels
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.FirRuleErrors
import dev.jasonpearson.krit.fir.incremental.CompileRecording
import dev.jasonpearson.krit.fir.incremental.CompileRecordingRegistry
import dev.jasonpearson.krit.fir.incremental.FingerprintBuilder
import dev.jasonpearson.krit.fir.incremental.OraclePayloadCache
import dev.jasonpearson.krit.fir.incremental.PersistedState
import dev.jasonpearson.krit.fir.incremental.PersistedVerdict
import dev.jasonpearson.krit.fir.incremental.BuiltInTieredPolicy
import dev.jasonpearson.krit.fir.incremental.BundleStore
import dev.jasonpearson.krit.fir.incremental.CachedVerdict
import dev.jasonpearson.krit.fir.incremental.IncrementalStage
import dev.jasonpearson.krit.fir.incremental.IncrementalState
import dev.jasonpearson.krit.fir.incremental.IncrementalTestHook
import dev.jasonpearson.krit.fir.incremental.SNAPSHOT_MODULE_NAME
import dev.jasonpearson.krit.fir.incremental.SliceFallback
import dev.jasonpearson.krit.fir.incremental.SliceOutcome
import dev.jasonpearson.krit.fir.incremental.PlanTelemetry
import dev.jasonpearson.krit.fir.incremental.PlannerLimits
import dev.jasonpearson.krit.fir.incremental.Snapshot
import dev.jasonpearson.krit.fir.incremental.SnapshotBuild
import dev.jasonpearson.krit.fir.incremental.SnapshotComponents
import dev.jasonpearson.krit.fir.incremental.SnapshotStorage
import dev.jasonpearson.krit.fir.incremental.TieredRulePolicy
import dev.jasonpearson.krit.fir.incremental.VerdictCache
import dev.jasonpearson.krit.fir.incremental.VerdictKeys
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
    /** Per file, the source-side annotations of its annotated callables; recorded only for incremental use. */
    val declaredAnnotations: Map<String, Map<String, List<String>>> = emptyMap(),
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
    // What the incremental planner made of this check; null when it did not run.
    val plan: PlanSummary? = null,
)

/**
 * How the incremental planner served a check, for the response: [path] is "incremental" or "full", [reason] why a
 * full check was needed (null when incremental), [dirtyFiles] the files its slices compiled.
 */
data class PlanSummary(
    val path: String, val reason: String?, val dirtyFiles: Int, val totalMillis: Long,
    /** More about [reason] where it helps the caller act: why no slice compile was available. */
    val detail: String? = null,
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
    /**
     * Serves [check] from [incrementalState] and its snapshot when possible (see [IncrementalPlanner]); needs
     * [incrementalRecording] and [incrementalSnapshot]. Off keeps `check` the whole-module compile it is without the
     * feature. Defaults to `KRIT_FIR_INCREMENTAL=1`.
     */
    val incrementalPlanning: Boolean = incrementalRecordingFromEnv(),
    /**
     * The project's Kotlin compiler plugins, registered after krit-fir's own on every non-module compilation (a
     * module compilation takes its module's own, see [ModuleCompilation]). Empty keeps today's behavior.
     */
    val compilerPlugins: CompilerPlugins = CompilerPlugins.EMPTY,
) {

    /** True when [check] goes through the planner. */
    internal val plannerEnabled: Boolean get() = incrementalPlanning && incrementalRecording && incrementalSnapshot

    /** How the planner serves tiered rules; the default is the safe full compile. */
    @Volatile internal var tieredRulePolicy: TieredRulePolicy = BuiltInTieredPolicy.FULL_IF_TIERED_ENABLED

    @Volatile internal var plannerLimits = PlannerLimits()

    /** What the latest planned [check] came to; null before the first one and when the planner is off. */
    @Volatile internal var lastPlan: PlanTelemetry? = null

    /** Per-file verdicts of the module, in step with [incrementalState] (see [VerdictCache]). Touched under [snapshotLock]. */
    internal val verdicts = VerdictCache()

    /**
     * Per-file oracle facts of the module, in step with [incrementalState] like [verdicts] (see [OraclePayloadCache]):
     * what lets a warm `analyzeWithCheck` answer the oracle without a whole-module compile. Touched under [snapshotLock].
     */
    internal val oraclePayloads = OraclePayloadCache()

    /** Whether the latest `analyzeWithCheck` answered the oracle from [oraclePayloads]; for tests and telemetry. */
    @Volatile internal var lastOracleFromCache = false
        private set

    /** Whole-module commits to [incrementalState] so far; lets the planner see whether a full compile replaced it. */
    @Volatile internal var fullCommits = 0
        private set

    private val planner by lazy { IncrementalPlanner(this) }

    /**
     * Persists the incremental state across processes when the caller supplied a directory (see
     * [configurePersistence]); null keeps everything in memory, as without the feature.
     */
    @Volatile internal var persistence: BundlePersistence? = null
        private set

    private val persistenceLock = Any()

    /**
     * Turns persistence of the incremental bundle on under [root] (`{repo}/.krit/fir-incremental`), or off with null
     * (no directory supplied, `--no-cache`): a session without a root neither reads nor writes a bundle. Needs the
     * planner. Switching roots flushes what was pending to the old one first.
     */
    fun configurePersistence(root: File?) {
        synchronized(persistenceLock) {
            val wanted = root?.takeIf { plannerEnabled }?.absoluteFile?.normalize()
            val current = persistence
            if (current?.store?.root == wanted) return
            // Moving to another root writes what was pending to the old one; turning it off (--no-cache) drops it.
            if (wanted == null) current?.discard() else current?.close()
            persistence = wanted?.let { BundlePersistence(this, BundleStore(it)) }
            // A state that is already warm (e.g. after requests that bypassed the cache) gets written eventually.
            if (persistence != null && incrementalState.compileKey != null) persistence?.markDirty(afterFullCheck = false)
        }
    }

    /** Writes any pending persisted bundle now; for shutdown and tests. */
    internal fun flushPersistence() { persistence?.flush() }

    /** The planner found the session cold: load the persisted bundle of its compile key, if there is one. Holds [snapshotLock]. */
    internal fun loadPersistedIfCold() {
        val persisted = persistence ?: return
        if (incrementalState.compileKey != null) return
        persisted.loadOnce()
    }

    /** The key of this session's compiler facts as of now; what a persisted bundle is filed under. */
    internal fun compileKeyNow(): String = compileKey()

    /** Parent of the snapshot directories, for tests; null uses the per-user krit cache dir. */
    internal var snapshotRootOverride: File? = null

    /** Lets tests adjust the compiler arguments of every non-module compile (Java source roots). */
    internal var argumentsHook: ((K2JVMCompilerArguments) -> Unit)? = null

    // Serializes whatever touches the snapshot directory: slice compiles (which move files in it), and the commits
    // of a whole-module compile (which replace and delete it).
    internal val snapshotLock = Any()

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
        try { resetIncremental() } catch (_: Throwable) {}
        if (!recordingFailureLogged) {
            recordingFailureLogged = true
            System.err.println("krit-fir: incremental recording failed and was dropped (${cause.javaClass.name}: ${cause.message})")
        }
    }

    /** Drops every recorded fact, the snapshot and the verdicts: the next planned check is a cold full check. */
    internal fun resetIncremental() {
        synchronized(snapshotLock) { incrementalState.reset(); verdicts.clear(); oraclePayloads.clear() }
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

    // The project's plugins as the embedded compiler can load them: probed once on first use, empty (with a warning)
    // when they cannot be loaded, so the session keeps the no-plugin behavior instead of failing every file.
    private val verifiedPlugins = lazy { compilerPlugins.verified(classpath) }

    /** The compiler plugins every non-module compilation of this session registers. */
    internal val effectivePlugins: CompilerPlugins get() = verifiedPlugins.value

    /** Why a supplied compiler plugin or option is not applied; surfaced in the response like [kotlinSettingsWarnings]. */
    val compilerPluginWarnings: List<String>
        get() = (if (verifiedPlugins.isInitialized()) verifiedPlugins.value.warnings else compilerPlugins.warnings)
            .map { "krit-fir: $it" }

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
        if (plannerEnabled) return planner.serve(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, fullCompile = null)
        return checkCompilation(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels)
    }

    internal fun checkCompilation(
        id: Long, files: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>>,
        testFiles: Set<String>, scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
        module: ModuleCompilation? = null,
        ownedSources: Set<String> = emptySet(),
        // False: a plain check that records nothing, builds no snapshot and leaves the incremental state alone.
        record: Boolean = true,
    ): BatchResult {
        val plan = CheckPlan(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, module, ownedSources)
        if (plan.compiled.isEmpty() && module == null) return plan.emptyResult()
        val outDir = module?.output ?: Files.createTempDirectory("krit-fir-out-").toFile()
        val exitCode = try {
            val sources = module?.sources ?: compilationFiles(plan.compiled.map { it.path })
            val args = compilationArguments(sources, outDir, module)
            // A session runs many module compiles; only the legacy whole-session compile is recorded.
            if (module == null && record) plan.startRecording(sources)
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
        val result = plan.result(exitCode)
        if (module == null && plannerEnabled && plan.committed) {
            storeFullVerdicts(plan, result)
            persistence?.markDirty(afterFullCheck = true)
        }
        return result
    }

    // The verdicts of a whole-module check that just replaced the state: every requested file's, and nothing else.
    private fun storeFullVerdicts(plan: CheckPlan, result: BatchResult) {
        try {
            synchronized(snapshotLock) {
                verdicts.clear()
                storeVerdicts(result, plan.compiled, plan.verdictKeys)
                verdicts.sync(incrementalState.generation)
            }
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            verdicts.clear()
        }
    }

    /**
     * Stores the oracle facts, `cacheDeps` edges and declared annotations [outcome] holds for each of [files]
     * (canonical paths) under its content hash in [hashes]; a file the compile gave no facts, or that has no hash,
     * loses its entry. False when a supertype edge names a class the state cannot place in a file: the caller drops
     * the cache. Holds [snapshotLock].
     */
    private fun storeOraclePayloads(
        outcome: AnalyzeOutcome, files: Collection<String>, hashes: Map<String, String>, wholeModule: Boolean = false,
    ): Boolean {
        val result = outcome.result
        val deps = outcome.cacheDeps
        fun byCanonical(map: Map<String, *>) = map.keys.associateBy { canonicalOrSelf(File(it)) }
        val payloadKeys = byCanonical(result.files)
        if (wholeModule) oraclePayloads.recordOrder(payloadKeys.keys)
        val depKeys = byCanonical(deps.depPathsByFile)
        val perFileKeys = byCanonical(deps.perFileDeps)
        val declaredKeys = byCanonical(outcome.declaredAnnotations)
        for (file in files) {
            val hash = hashes[file]
            val payload = payloadKeys[file]?.let { result.files[it] }
            if (hash == null || payload == null) { oraclePayloads.remove(file); continue }
            val supertypes = LinkedHashMap<String, String>()
            for (fqn in perFileKeys[file]?.let { deps.perFileDeps[it] }.orEmpty().keys) {
                supertypes[fqn] = incrementalState.fileDeclaring(fqn) ?: return false
            }
            oraclePayloads.put(file, OraclePayloadCache.Entry(
                hash, payload,
                depPaths = depKeys[file]?.let { deps.depPathsByFile[it] }.orEmpty().map { canonicalOrSelf(File(it)) },
                perFileDeps = supertypes,
                declaredAnnotations = declaredKeys[file]?.let { outcome.declaredAnnotations[it] }.orEmpty(),
            ))
        }
        return true
    }

    // Splits [result] into the verdict of each compiled file, keyed by canonical path and the state's content hash.
    private fun storeVerdicts(result: BatchResult, compiled: List<FileRef>, keys: VerdictKeys) {
        val findings = result.findings.groupBy { canonicalOrSelf(File(it.path)) }
        val errors = result.errorFiles.entries.associate { canonicalOrSelf(File(it.key)) to it.value }
        val ruleErrors = HashMap<String, MutableMap<String, String>>()
        for ((rule, byPath) in result.ruleErrors) {
            for ((path, message) in byPath) ruleErrors.getOrPut(canonicalOrSelf(File(path))) { HashMap() }[rule] = message
        }
        for (ref in compiled) {
            val file = canonicalOrSelf(File(ref.path))
            val hash = incrementalState.contentHash(file)
            if (hash == null) { verdicts.remove(file); continue }
            verdicts.put(
                file,
                VerdictCache.Entry(
                    hash, ref.contentHash.takeIf { it.isNotBlank() }, keys.of(file),
                    CachedVerdict(findings[file].orEmpty(), errors[file], ruleErrors[file].orEmpty()),
                ),
            )
        }
    }

    private var lastKeys: Pair<List<Any>, VerdictKeys>? = null

    // The same request inputs (by identity) give the same keys: a planner's waves share one canonicalisation.
    internal fun verdictKeysFor(
        enabledRules: Set<String>, ruleConfigs: Map<String, Map<String, Any?>>,
        testFiles: Set<String>, scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
    ): VerdictKeys {
        val inputs = listOf<Any>(enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels)
        lastKeys?.let { (seen, keys) -> if (seen.indices.all { seen[it] === inputs[it] }) return keys }
        return VerdictKeys(enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels).also { lastKeys = inputs to it }
    }

    /** The files [check] would compile for [files], and the part of its result that does not depend on the compile. */
    internal class RequestShape(val compiled: List<FileRef>, val skeleton: BatchResult)

    internal fun shapeOf(
        id: Long, files: List<FileRef>, enabledRules: Set<String>, ruleConfigs: Map<String, Map<String, Any?>>,
        testFiles: Set<String>, scanPaths: Map<String, String>, sdkLevels: Map<String, SdkLevels>,
    ): RequestShape {
        val plan = CheckPlan(id, files, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, null, emptySet())
        return RequestShape(plan.compiled, plan.emptyResult())
    }

    /** The module's files for a request that names [requested]: the walked sources plus those, canonical paths. */
    internal fun moduleFilesFor(requested: List<String>): Set<String> =
        compilationFiles(requested).mapTo(LinkedHashSet()) { canonicalOrSelf(File(it)) }

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

    /** [sliceFallback] for telemetry: its name, plus what the state knows about it (`COMPILE_ERRORS: compile errors in 6 files`). */
    internal fun sliceFallbackDescription(): String? {
        val fallback = sliceFallback() ?: return null
        val detail = incrementalState.fallbackDetail().takeIf { fallback == incrementalState.fallbackReason() }
        return if (detail == null) fallback.name else "${fallback.name}: $detail"
    }

    /** Test seam: a slice withholds the external class-name facts, which must make the rule differ from a full compile. */
    @Volatile internal var withheldExternalFacts = false

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
     *  - the whole sealed hierarchy of every sealed class or interface a compiled (or deleted) file touches: the
     *    inheritors of a sealed class it declares, and the file of a sealed parent it extends, recorded or
     *    current, so a subclass that is added, removed or retargeted brings its parent and the parent's other
     *    inheritors in. A sealed parent compiled from source knows only the inheritors that are in source, so
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
     *  - the dependents of what changed (lookups, plus the structural rules of the findings): a `when` over a
     *    sealed hierarchy that gained or lost a subclass is such a dependent and is not added here;
     *  - a redeclaration or conflicting overload is only seen when both sides are in the slice;
     *  - verdicts of the whole-compilation tiered rules (`FirRule.wholeCompilationTier`) are not those of a full
     *    compile on a slice and must not be trusted; step 4 decides how they are served;
     *  - SerialVersionUIDInSerializableClass is served on a slice: the recorded class-name facts of the module files
     *    outside the slice reach it through `FirRuleCompileContext.externalClassNameFacts`. A slice that moves the
     *    set of names reaching `Serializable` (`IncrementalState.serializableNames`) makes verdicts outside it
     *    stale, which the planner detects;
     *  - a module that uses `expect`/`actual` has no snapshot ([SliceFallback.EXPECT_ACTUAL]).
     */
    internal fun sliceCheck(
        id: Long, dirtyFiles: List<FileRef>, enabledRules: Set<String>,
        ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        sdkLevels: Map<String, SdkLevels> = emptyMap(),
        removedFiles: Collection<String> = emptyList(),
        spellings: Map<String, FileRef> = emptyMap(),
        withOracle: Boolean = false,
    ): SliceOutcome = synchronized(snapshotLock) {
        sliceFallback()?.let { return SliceOutcome.Fallback(it) }
        val snapshot = incrementalState.snapshot ?: return SliceOutcome.Fallback(SliceFallback.NO_SNAPSHOT)
        val refs = LinkedHashMap<String, FileRef>()
        for (ref in dirtyFiles) refs.putIfAbsent(canonicalOrSelf(File(ref.path)), ref)
        val requested = refs.keys.toSet()
        for (stale in incrementalState.binaryStale()) refs.putIfAbsent(stale, FileRef(stale))
        var wanted = incrementalState.withSealedInheritors(refs.keys + removedFiles.map { canonicalOrSelf(File(it)) })
        while (true) {
            // A deleted file that is still on disk (it left the module, e.g. no longer requested) is not compiled.
            val deleted = removedFiles.mapTo(HashSet()) { canonicalOrSelf(File(it)) }
            val (present, missing) = wanted.partition { File(it).isFile && it !in deleted }
            val removed = (removedFiles + missing).mapTo(LinkedHashSet()) { canonicalOrSelf(File(it)) }
            // The module files this slice reads as binaries have no source for a rule that indexes classes by name:
            // hand it their recorded facts. Hidden files (deleted since) are not part of the module any more.
            val external = incrementalState.classFactsExcluding(present.map { canonicalOrSelf(File(it)) } + removed + removedFiles.map { canonicalOrSelf(File(it)) })
            val plan = CheckPlan(
                id, present.map { refs[it] ?: spellings[it] ?: FileRef(it) }, enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels, null, emptySet(),
                externalClassNameFacts = external.takeUnless { withheldExternalFacts },
            )
            val sources = plan.compiled.map { it.path }
            plan.startRecording(sources)
            val recording = plan.recording ?: return SliceOutcome.Fallback(SliceFallback.NO_SNAPSHOT)
            val compiled = sources.mapTo(LinkedHashSet()) { canonicalOrSelf(File(it)) }
            if (sources.isEmpty()) {
                // Only deletions (or files krit never compiles): nothing to compile.
                if (removed.isNotEmpty()) {
                    mergeSlice(plan, compiled, removed, emptySet(), plan.compiled, plan.emptyResult(), OracleCollector().takeIf { withOracle })
                }
                return SliceOutcome.Checked(plan.emptyResult(), compiled, compiled - requested)
            }
            val outDir = Files.createTempDirectory("krit-fir-slice-out-").toFile()
            try {
                val args = compilationArguments(sources, outDir)
                args.incrementalCompilation = true
                args.moduleName = SNAPSHOT_MODULE_NAME
                args.classpath = listOfNotNull(args.classpath, snapshot.dir.path).joinToString(File.pathSeparator)
                val hidden = sources + removed + incrementalState.removedSinceSnapshot()
                // The oracle's facts for the compiled files, from the same frontend run (#740): registered as
                // analyzeCompilation registers them for the shared compile.
                val oracle = if (withOracle) OracleCollector() else null
                if (oracle != null) {
                    oracle.recordDeclarations = true
                    oracle.binarySupertype = { fqn ->
                        incrementalState.fileDeclaring(fqn)?.let { file ->
                            oraclePayloads.declaration(file, fqn)?.let { file to it }
                                ?: run { oracle.binarySupertypeMissed = true; null }
                        }
                    }
                    val declared by lazy { oraclePayloads.declaredAnnotationIndex(compiled) }
                    oracle.binaryCalleeAnnotations = { signature -> declared[signature] }
                }
                val messages = listOfNotNull(
                    oracle?.let { o -> OracleDiagnosticMessageCollector(o, args.freeArgs.associateBy { File(it).canonicalPath }) },
                    plan.collector,
                )
                val contexts = listOfNotNull(
                    oracle?.let { o -> CompilationContext({ OracleCollectorRegistry.begin(o) }, { OracleCollectorRegistry.end() }) },
                ) + plan.contexts()
                val exit = try {
                    snapshot.parked(hidden) {
                        compileModule(args, messages, contexts, frontendOnly = true, recording = recording, slice = snapshot)
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
                val grown = incrementalState.withSealedInheritors(compiled + removed, recording.surfaces)
                if ((grown - compiled - removed).any { File(it).isFile }) {
                    wanted = grown + wanted
                    continue
                }
                val result = plan.result(exit)
                val errored = result.errorFiles.keys.mapTo(HashSet()) { canonicalOrSelf(File(it)) }
                mergeSlice(plan, compiled, removed, errored, plan.compiled, result, oracle)
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
    private fun mergeSlice(
        plan: CheckPlan, compiled: Set<String>, removed: Set<String>, errored: Set<String>,
        compiledRefs: List<FileRef>, result: BatchResult, oracle: OracleCollector? = null,
    ) {
        try {
            val recorded = plan.recording
            if (recorded == null || recorded.failed) {
                recordingFailed(recorded?.failure ?: IllegalStateException("recording failed"))
            } else {
                // The verdict cache follows the state only while it was in step with it before this commit.
                val inStep = plannerEnabled && verdicts.generation == incrementalState.generation
                // The oracle facts follow the state the same way; a slice that did not run the oracle leaves them
                // a generation behind, which makes them unusable until a whole-module oracle compile refills them.
                val oracleInStep = oracle != null && plannerEnabled && oraclePayloads.generation == incrementalState.generation
                incrementalState.commitSlice(recorded, compiled, removed, errored)
                if (inStep) {
                    for (file in removed) verdicts.remove(file)
                    storeVerdicts(result, compiledRefs, plan.verdictKeys)
                    verdicts.sync(incrementalState.generation)
                }
                if (oracleInStep) {
                    for (file in removed) oraclePayloads.remove(file)
                    val stored = !oracle.binarySupertypeMissed && !oracle.binaryCalleeAmbiguous && storeOraclePayloads(
                        AnalyzeOutcome(oracle.toResult(), oracle.depTracker.view(), oracle.declaredAnnotations), compiled,
                        incrementalState.contentHashes(),
                    )
                    if (stored) oraclePayloads.sync(incrementalState.generation) else oraclePayloads.clear()
                }
                persistence?.markDirty(afterFullCheck = false)
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
        private val ruleConfigs: Map<String, Map<String, Any?>>,
        private val testFiles: Set<String>, private val scanPaths: Map<String, String>, private val sdkLevels: Map<String, SdkLevels>,
        module: ModuleCompilation?,
        ownedSources: Set<String>,
        externalClassNameFacts: List<ClassNameFact>? = null,
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
            sdkLevels = sdkLevels, externalClassNameFacts = externalClassNameFacts,
        )

        // Incremental recording of this compile; null unless the session records.
        var recording: CompileRecording? = null
            private set
        private val requestVerdictKey: String by lazy { verdictKey(enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels) }

        /** Per-file verdict keys of this request. */
        val verdictKeys: VerdictKeys by lazy { verdictKeysFor(enabledRules, ruleConfigs, testFiles, scanPaths, sdkLevels) }

        /** True once this plan's recording replaced the state. */
        var committed = false
            private set

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
                        verdicts.clear()
                        oraclePayloads.clear()
                        fullCommits++
                        committed = true
                        val snapshot = build?.takeIf { it.built }?.snapshot
                        // A compile with errors never reaches the backend: say so, with how many files, so the
                        // planner's fallback is visible as more than "no snapshot" (a missing compiler plugin, say).
                        val erroneous = build != null && !build.built && !build.frontendClean && build.failure == null
                        if (erroneous) {
                            val files = collector.errorFiles.size
                            val detail = if (files > 0) "compile errors in $files file${if (files == 1) "" else "s"}"
                                else "a compiler error not tied to a file"
                            incrementalState.attachSnapshot(null, SliceFallback.COMPILE_ERRORS, detail)
                        } else {
                            incrementalState.attachSnapshot(snapshot, noSnapshotReason)
                        }
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
     * The first file a persisted bundle names that is neither under this session's source directories nor under
     * [repoDir] (the repository the bundle directory belongs to), or null. Go builds the source directories from the
     * `kotlin`/`java` directories below the scan roots, while the requested files are every `.kt` the scan collected,
     * so ordinary runs have files in the state that are outside the former and inside the latter. The state's paths are
     * canonical, as are the roots. Verdict files and the paths of their findings must be files of the state, or lie
     * under the roots (they may be spelled as the request spelled them).
     */
    internal fun pathOutsideSources(state: PersistedState, verdicts: List<PersistedVerdict>, repoDir: File?): String? {
        val roots = (canonicalSourceDirs + listOfNotNull(repoDir?.let { canonicalOrSelf(it) })).map { java.nio.file.Path.of(it) }
        fun outside(path: String): Boolean {
            val p = try { java.nio.file.Path.of(path).normalize() } catch (_: Exception) { return true }
            return !p.isAbsolute || roots.none { p.startsWith(it) }
        }
        val snap = state.snapshot
        for (paths in listOf(
            state.contentHashes.keys, state.surfaces.keys, state.classFacts.keys, state.lookups.keys,
            snap.outputs.keys, snap.surfaceHashes.keys, snap.binaryStale, snap.removed,
        )) for (path in paths) if (outside(path)) return path
        // Verdicts are keyed by the spelling of the request (Go's absolute, not canonical, paths), so they are resolved
        // before they are compared with the canonical roots and state; each distinct spelling once.
        val resolved = HashMap<String, String>()
        fun foreign(path: String): Boolean {
            val canonical = resolved.getOrPut(path) { canonicalOrSelf(File(path)) }
            return canonical !in state.contentHashes && outside(canonical)
        }
        for (v in verdicts) {
            if (foreign(v.file)) return v.file
            for (f in v.entry.verdict.findings) if (foreign(f.path)) return f.path
        }
        return null
    }

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
     *
     * With the incremental planner on (#880), the check goes through the planner and only its full compiles are
     * shared: the planner's full check becomes this shared compile, which then also records the incremental state,
     * builds the binary snapshot (the backend runs after the frontend that produced both the oracle facts and the
     * verdicts) and persists the bundle under the request's `incrementalDir`. So a cold scan costs one compile plus
     * the snapshot backend, and a warm one is served from slices. The slices run the oracle too, and the oracle is
     * answered from the per-file facts they and the full checks recorded ([oraclePayloads], #740); only when those
     * are missing or out of step (a restarted process, a check-only slice) does the oracle compile alone (frontend
     * only, no checkers), which refills them. Without the planner the shared compile records nothing and leaves the
     * state as it is.
     */
    fun analyzeWithCheck(files: List<String>, checkRequest: CheckRequest): Pair<AnalyzeOutcome, BatchResult?> {
        lastOracleFromCache = false
        val walked = currentSourceFiles()
        val oracleSources = compilationFiles(files, walked)
        val rules = checkRequest.rules.toSet()
        fun planFor(enabledRules: Set<String>) = CheckPlan(
            checkRequest.id, checkRequest.files, enabledRules, checkRequest.ruleConfigs,
            checkRequest.testFiles, checkRequest.scanPaths, checkRequest.sdkLevels, module = null, ownedSources = emptySet(),
        )
        val probe = planFor(rules)
        val shared = probe.compiled.isNotEmpty() &&
            sharesCompileContext(checkRequest) &&
            canonicalSet(oracleSources) == canonicalSet(compilationFiles(probe.compiled.map { it.path }, walked))
        if (!shared) return analyzeFull(files) to null
        if (!plannerEnabled) return sharedCompile(files, oracleSources, probe, record = false)

        configurePersistence(BundleStore.trustedRoot(checkRequest.incrementalDir))
        var sharedOutcome: AnalyzeOutcome? = null
        val checked = try {
            planner.serve(
                checkRequest.id, checkRequest.files, rules, checkRequest.ruleConfigs,
                checkRequest.testFiles, checkRequest.scanPaths, checkRequest.sdkLevels, withOracle = true,
            ) { enabledRules, record ->
                val (outcome, result) = sharedCompile(files, oracleSources, planFor(enabledRules), record)
                // A shared compile that fell back to the oracle alone has no check: the planner's full check
                // must not be served from it, so compile the check on its own as a plain full check would.
                sharedOutcome = outcome
                result ?: checkCompilation(
                    checkRequest.id, checkRequest.files, enabledRules, checkRequest.ruleConfigs,
                    checkRequest.testFiles, checkRequest.scanPaths, checkRequest.sdkLevels, record = record,
                )
            }
        } catch (e: Exception) {
            if (!isIsolatable(e)) throw e
            System.err.println("krit-fir: incremental shared check failed (${e.message}); compiling the oracle alone")
            return (sharedOutcome ?: analyzeFull(files)) to null
        }
        // Served from slices (or cached verdicts): the slices ran the oracle too, so its facts come from the cache;
        // without them (a cold cache, or one a check-only slice left behind) it compiles on its own.
        return (sharedOutcome ?: oracleFromCache(oracleSources) ?: analyzeFullRefilling(files, oracleSources)) to checked
    }

    /**
     * The oracle answer for [sources] from the cached facts, when they are in step with the state and every file has
     * an entry for the content the state recorded; null otherwise (#740).
     */
    private fun oracleFromCache(sources: List<String>): AnalyzeOutcome? = synchronized(snapshotLock) {
        if (incrementalState.compileKey == null || oraclePayloads.generation != incrementalState.generation) return null
        val spelled = LinkedHashMap<String, String>()
        for (path in sources) spelled.putIfAbsent(canonicalOrSelf(File(path)), path)
        val assembled = oraclePayloads.assemble(spelled, incrementalState.contentHashes()) ?: return null
        lastOracleFromCache = true
        AnalyzeOutcome(assembled.result, assembled.cacheDeps)
    }

    /**
     * [analyzeFull], which also refills the cached oracle facts when the state describes exactly the content it
     * compiled: a cache that is out of step (a restarted process, check-only slices) is then current again.
     */
    private fun analyzeFullRefilling(files: List<String>, sources: List<String>): AnalyzeOutcome {
        val before = contentHashesOf(sources)
        val outcome = analyzeFull(files)
        try {
            synchronized(snapshotLock) {
                val recorded = incrementalState.contentHashes()
                if (plannerEnabled && incrementalState.compileKey != null && recorded == before && contentHashesOf(sources) == before) {
                    oraclePayloads.clear()
                    if (storeOraclePayloads(outcome, before.keys, recorded, wholeModule = true)) oraclePayloads.sync(incrementalState.generation)
                    else oraclePayloads.clear()
                }
            }
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            synchronized(snapshotLock) { oraclePayloads.clear() }
        }
        return outcome
    }

    /** The oracle facts of a shared full check that just replaced the state. Holds [snapshotLock]. */
    private fun storeFullOracle(outcome: AnalyzeOutcome, sources: List<String>) {
        try {
            synchronized(snapshotLock) {
                oraclePayloads.clear()
                val stored = storeOraclePayloads(outcome, sources.map { canonicalOrSelf(File(it)) }, incrementalState.contentHashes(), wholeModule = true)
                if (stored) oraclePayloads.sync(incrementalState.generation) else oraclePayloads.clear()
            }
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            synchronized(snapshotLock) { oraclePayloads.clear() }
        }
    }

    /**
     * One compile for the oracle and [plan]'s check. With [record] (the planner's full check) it also records the
     * incremental state and builds the snapshot, as [checkCompilation] does. A compile that throws gives the oracle a
     * compile of its own and no check result, so the caller checks on its own.
     */
    private fun sharedCompile(
        files: List<String>, oracleSources: List<String>, plan: CheckPlan, record: Boolean,
    ): Pair<AnalyzeOutcome, BatchResult?> {
        var exitCode = ExitCode.OK
        val outcome = try {
            analyzeCompilation(oracleSources, plan, record) { exitCode = it }
        } catch (e: Exception) {
            // A checker failure the rule isolation does not absorb must not
            // cost the oracle its facts: compile for the oracle alone, and
            // let the caller's own check meet the failure as before.
            System.err.println("krit-fir: shared oracle/check compile failed (${e.message}); compiling the oracle alone")
            return analyzeFull(files) to null
        }
        val result = plan.result(exitCode)
        if (record && plannerEnabled && plan.committed) {
            storeFullVerdicts(plan, result)
            storeFullOracle(outcome, oracleSources)
            persistence?.markDirty(afterFullCheck = true)
        }
        return outcome to result
    }

    private fun sharesCompileContext(request: CheckRequest): Boolean {
        fun sameFiles(requested: List<String>, own: List<String>) =
            requested.isEmpty() || requested.map { canonicalOrSelf(File(it)) } == own.map { canonicalOrSelf(File(it)) }
        return sameFiles(request.sourceDirs, sourceDirs) && sameFiles(request.classpath, classpath) &&
            (request.jvmTarget.isEmpty() || request.jvmTarget == jvmTarget) &&
            (request.kotlinArgs.isEmpty() || KotlinCompilerSettings.parse(request.kotlinArgs).identity == kotlinSettings.identity) &&
            (request.compilerPluginClasspath.isEmpty() || request.compilerPlugins().identity == compilerPlugins.identity)
    }

    private fun canonicalSet(paths: List<String>): Set<String> = paths.mapTo(HashSet()) { canonicalOrSelf(File(it)) }

    private fun analyzeCompilation(
        sources: List<String>, check: CheckPlan?, record: Boolean = false, onExit: (ExitCode) -> Unit = {},
    ): AnalyzeOutcome {
        val collector = OracleCollector().apply { recordDeclarations = plannerEnabled }
        val outDir = Files.createTempDirectory("krit-fir-oracle-out-").toFile()
        try {
            val args = compilationArguments(sources, outDir)
            // Taken before the snapshot build retargets the output and module name.
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
                if (record) check.startRecording(sources)
                val build = if (record) check.prepareSnapshot(args, sources) else null
                try {
                    compileModule(args, listOf(oracleMessages, check.collector), listOf(oracleContext) + check.contexts(), frontendOnly = true,
                        recording = check.recording, build = build).also { check.commitRecording(it, build) }
                } finally {
                    check.discardSnapshot()
                }
            })
        } finally {
            outDir.deleteRecursively()
        }
        return AnalyzeOutcome(
            result = collector.toResult(), cacheDeps = collector.depTracker.view(), declaredAnnotations = collector.declaredAnnotations,
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
        (module?.plugins ?: effectivePlugins).applyTo(this, selfJar)
        module?.configure(this, compilationJvmTarget)
        if (module == null) argumentsHook?.invoke(this)
    }

    /**
     * Key of the compiler facts (lookups, surfaces): the compiler and plugin builds, this session's sources,
     * classpath (with file identity), JVM target, Kotlin settings and the project's compiler plugins (each jar's file
     * identity and every option). A different key resets [incrementalState] and files a persisted bundle elsewhere.
     */
    private fun compileKey(): String {
        val builder = FingerprintBuilder()
            .add("kotlin", KotlinCompilerVersion.VERSION)
            .add("sourceDirs", sourceDirs.map { canonicalOrSelf(File(it)) })
            .add("jvmTarget", compilationJvmTarget)
            .add("kotlinSettings", kotlinSettings.identity)
        selfJar?.let { builder.addFileIdentity("plugin", it) }
        effectivePlugins.addTo(builder)
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
        compilerPlugins: CompilerPlugins = this.compilerPlugins,
    ): AnalysisSession =
        AnalysisSession(
            sourceDirs, classpath, jvmTarget, kotlinArgs, incrementalRecording, incrementalSnapshot, incrementalPlanning,
            compilerPlugins,
        ).also {
            it.tieredRulePolicy = tieredRulePolicy
            it.plannerLimits = plannerLimits
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
        // Writes the bundle of a state still waiting for its debounced write, before the state goes.
        try { synchronized(persistenceLock) { persistence?.close(); persistence = null } } catch (_: Throwable) {}
        // Also deletes the snapshot directory.
        resetIncremental()
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
