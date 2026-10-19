@file:OptIn(org.jetbrains.kotlin.fir.SessionConfiguration::class, org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package spike

import com.intellij.openapi.Disposable
import com.intellij.openapi.util.Disposer
import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.FirRuleErrors
import dev.jasonpearson.krit.fir.runner.BundledStdlib
import dev.jasonpearson.krit.fir.runner.FindingCollector
import dev.jasonpearson.krit.fir.runner.KotlinCompilerSettings
import dev.jasonpearson.krit.fir.runner.resolveJvmTarget
import org.jetbrains.kotlin.backend.common.phaser.then
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.allFiles
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.collectSources
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.VfsBasedProjectEnvironment
import org.jetbrains.kotlin.cli.pipeline.AbstractCliPipeline
import org.jetbrains.kotlin.cli.pipeline.ArgumentsPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.CheckCompilationErrors
import org.jetbrains.kotlin.cli.pipeline.ConfigurationPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PerformanceNotifications
import org.jetbrains.kotlin.cli.pipeline.PipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PipelineContext
import org.jetbrains.kotlin.cli.pipeline.PipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmBackendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmConfigurationPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFir2IrPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmWriteOutputsPhase
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi
import org.jetbrains.kotlin.compiler.plugin.getCompilerExtensions
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.config.Services
import org.jetbrains.kotlin.cli.common.diagnosticsCollector
import org.jetbrains.kotlin.config.languageVersionSettings
import org.jetbrains.kotlin.config.moduleName
import org.jetbrains.kotlin.config.phaser.CompilerPhase
import org.jetbrains.kotlin.fir.DependencyListForCliModule
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.FirSourceModuleData
import org.jetbrains.kotlin.fir.analysis.checkers.CliOnlyLanguageVersionSettingsCheckers
import org.jetbrains.kotlin.fir.extensions.FirExtensionRegistrar
import org.jetbrains.kotlin.fir.pipeline.AllModulesFrontendOutput
import org.jetbrains.kotlin.fir.pipeline.SingleModuleFrontendOutput
import org.jetbrains.kotlin.fir.pipeline.buildFirViaLightTree
import org.jetbrains.kotlin.fir.pipeline.resolveAndCheckFir
import org.jetbrains.kotlin.fir.pipeline.runPlatformCheckers
import org.jetbrains.kotlin.fir.resolve.providers.impl.FirBuiltinSyntheticFunctionInterfaceProvider
import org.jetbrains.kotlin.fir.resolve.providers.impl.syntheticFunctionInterfacesSymbolProvider
import org.jetbrains.kotlin.fir.session.FirJvmSessionFactory
import org.jetbrains.kotlin.fir.session.IncrementalCompilationContext
import org.jetbrains.kotlin.fir.session.KmpModuleKind
import org.jetbrains.kotlin.fir.session.createSymbolProviders
import org.jetbrains.kotlin.fir.session.environment.AbstractProjectFileSearchScope
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.util.PerformanceManager
import org.jetbrains.kotlin.util.PerformanceManagerImpl
import java.io.File

/** The krit FIR plugin jar (the plain `:jar` output), as AnalysisSession resolves it in tests. */
val pluginJar: String = System.getProperty("krit.fir.plugin.jar") ?: error("-Dkrit.fir.plugin.jar is required")

fun canon(p: String): String = try { File(p).canonicalPath } catch (_: Exception) { File(p).absolutePath }

/**
 * Compiler arguments equal to `AnalysisSession.compilationArguments` for a non-module compile, plus an explicit
 * module name (incremental slices must share it with the snapshot so `internal` resolves) and optional extra
 * classpath entries (the binary snapshot directory).
 */
fun kritArgs(
    sources: List<String>, classpath: List<String>, out: File, moduleName: String = "main",
    kotlinArgs: List<String> = emptyList(), extraClasspath: List<String> = emptyList(),
): K2JVMCompilerArguments = K2JVMCompilerArguments().apply {
    freeArgs = sources
    this.classpath = (BundledStdlib.effectiveClasspath(classpath) + extraClasspath).joinToString(File.pathSeparator)
    jvmTarget = resolveJvmTarget("")
    KotlinCompilerSettings.parse(kotlinArgs).applyTo(this)
    destination = out.absolutePath
    noStdlib = true
    noReflect = true
    suppressWarnings = false
    reportAllWarnings = true
    pluginClasspaths = arrayOf(pluginJar)
    this.moduleName = moduleName
}

/** One file's verdict as Go consumes it, plus every compiler diagnostic for the strict comparison. */
data class FileVerdict(
    val findings: List<String>,
    val error: Boolean,
    val ruleErrors: Map<String, String>,
    val diags: List<String>,
) {
    fun verdictKey() = Triple(findings, error, ruleErrors.keys.sorted())
}

class CheckOut(
    val verdicts: Map<String, FileVerdict>,
    val exit: ExitCode,
    val firstError: String?,
    val globalError: String?,
    val crashed: String?,
    val outputs: Map<String, List<String>> = emptyMap(),
)

/** Collects every located diagnostic per canonical file path (strict comparison) and OUTPUT messages. */
class AllDiags : MessageCollector {
    val byFile = HashMap<String, MutableList<String>>()
    val outputMessages = ArrayList<String>()
    override fun clear() {}
    override fun hasErrors() = false
    override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) {
        if (severity == CompilerMessageSeverity.OUTPUT) { outputMessages += message; return }
        if (location == null) return
        if (severity !in setOf(CompilerMessageSeverity.ERROR, CompilerMessageSeverity.WARNING, CompilerMessageSeverity.STRONG_WARNING)) return
        byFile.getOrPut(canon(location.path)) { ArrayList() }.add("${severity.name[0]}|${location.line}:${location.column}|$message")
    }
}

class Fanout(private val cs: List<MessageCollector>) : MessageCollector {
    override fun clear() = cs.forEach { it.clear() }
    override fun hasErrors() = cs.any { it.hasErrors() }
    override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) =
        cs.forEach { it.report(severity, message, location) }
}

/**
 * The krit side of one check compile, as `AnalysisSession.CheckPlan` sets it up: the rule context and the armed
 * rule-error recorder on the calling thread, a [FindingCollector] over the requested files.
 */
class KritCheck(requested: Collection<String>, val rules: Set<String> = emptySet()) {
    private val requestedCanon: Map<String, String> = requested.associate { canon(it) to canon(it) }
    val findings = FindingCollector(requestedCanon, rules)
    val all = AllDiags()
    private val recorder = FirRuleErrorRecorder(null)
    private val ctx = FirRuleCompileContext(rules, files = requested.toCollection(LinkedHashSet()))
    val collector: MessageCollector = Fanout(listOf(findings, all))

    fun <T> run(body: (MessageCollector) -> T): T {
        FirRuleContext.begin(ctx)
        FirRuleErrors.begin(recorder)
        try {
            return body(collector)
        } finally {
            FirRuleErrors.end()
            FirRuleContext.end()
        }
    }

    fun out(exit: ExitCode): CheckOut {
        val ruleErrs = HashMap<String, MutableMap<String, String>>() // file -> rule -> msg
        for ((rule, byPath) in recorder.snapshot()) for ((p, m) in byPath) {
            val targets = if (p.isEmpty()) requestedCanon.keys else listOfNotNull(requestedCanon[canon(p)])
            for (t in targets) ruleErrs.getOrPut(t) { HashMap() }.putIfAbsent(rule, m)
        }
        val global = findings.globalErrors.firstOrNull()
        val byFile = findings.findings.groupBy { it.path }
        val verdicts = requestedCanon.keys.associateWith { p ->
            FileVerdict(
                findings = byFile[p].orEmpty().map { "${it.rule}@${it.line}:${it.col} ${it.message}" }.sorted(),
                error = p in findings.errorFiles || global != null,
                ruleErrors = ruleErrs[p].orEmpty(),
                diags = all.byFile[p].orEmpty().sorted(),
            )
        }
        return CheckOut(verdicts, exit, findings.firstError, global, findings.exceptions.firstOrNull(), parseOutputs(all.outputMessages))
    }

    private fun parseOutputs(msgs: List<String>): Map<String, List<String>> {
        val m = HashMap<String, MutableList<String>>()
        for (msg in msgs) {
            val o = org.jetbrains.kotlin.cli.common.messages.OutputMessageUtil.parseOutputMessage(msg) ?: continue
            for (s in o.sourceFiles) m.getOrPut(canon(s.path)) { ArrayList() }.add(o.outputFile!!.path)
        }
        return m
    }
}

/** A CLI pipeline over an arbitrary phase chain, executed like `K2JVMCompiler` / `FrontendOnlyJvmPipeline`. */
class SpikePipeline(
    private val phase: CompilerPhase<PipelineContext, ArgumentsPipelineArtifact<K2JVMCompilerArguments>, *>,
) : AbstractCliPipeline<K2JVMCompilerArguments>() {
    override val defaultPerformanceManager: PerformanceManager =
        PerformanceManagerImpl(JvmPlatforms.defaultJvmPlatform, "spike")
    override fun createCompoundPhase(arguments: K2JVMCompilerArguments) = phase
}

/** Wall-clock split of one compile, filled by the warm phase. */
class Timings {
    var env = 0L; var libs = 0L; var session = 0L; var raw = 0L; var resolveCheck = 0L
    override fun toString() = "env=${ms(env)} libs=${ms(libs)} session=${ms(session)} raw=${ms(raw)} resolve+check=${ms(resolveCheck)}"
}

/**
 * Daemon-lifetime state for F1: the project environment (VFS, jar file system, classpath roots index, Java file
 * manager) and the shared + classpath library FIR sessions, kept across compiles. Only the source session is rebuilt.
 */
class WarmState(val reuseEnv: Boolean = true, val reuseLibs: Boolean = true) {
    val disposable: Disposable = Disposer.newDisposable("spike-warm-state")
    var env: VfsBasedProjectEnvironment? = null
    var libraryList: DependencyListForCliModule? = null
    var shared: FirSession? = null
    var context: FirJvmSessionFactory.Context? = null
    var registrars: List<FirExtensionRegistrar> = emptyList()
    /**
     * The LanguageVersionSettings instance the library sessions were built with. Library declarations cache
     * deprecation info keyed by the use-site LanguageVersionSettings (DeprecationsProviderImpl); a new instance per
     * compile adds a cache entry that retains that compile's configuration -> diagnostics -> source session.
     */
    var lvs: org.jetbrains.kotlin.config.LanguageVersionSettings? = null
    var pinLvs: Boolean = true
    /**
     * FirSharableJavaComponents (Java enhancement and JVM-mapped-scope caches) are shared by every session built from
     * one FirJvmSessionFactory.Context: meant for the sessions of one compilation. Reused across compiles they cache
     * symbols bound to an old source session. true: give each compile's source session fresh components.
     */
    var freshJavaComponents: Boolean = true
    /** Configurations the reused state keeps reachable (environment, pinned LVS): their diagnostics must be dropped. */
    val heldConfigurations = ArrayList<CompilerConfiguration>()

    /** Call after each compile: drops the diagnostics (and so the FIR) of compiles the reused state still references. */
    fun releaseHeldCompiles() {
        for (c in heldConfigurations) c.diagnosticsCollector = org.jetbrains.kotlin.diagnostics.impl.DiagnosticsCollectorImpl()
    }
    /** The binary snapshot directory, when there is one: never part of the libraries scope. */
    var snapshotDir: File? = null
    var envBuilds = 0
    var libBuilds = 0
    fun dispose() { Disposer.dispose(disposable) }
}

/**
 * Precompiled same-module binaries for a slice compile: [dir] is the snapshot class directory (it must be on the
 * compile classpath), [moduleMapping] the snapshot's `.kotlin_module` bytes.
 */
class Precompiled(val dir: File, val excluded: () -> Collection<File> = { emptyList() }, val moduleMapping: () -> ByteArray?)

/**
 * `JvmFrontendPipelinePhase.executePhase` with the environment and library sessions taken from [st] when present.
 * The source session, raw FIR, resolution and checkers are exactly the CLI's (same factories, same checker set,
 * `CliOnlyLanguageVersionSettingsCheckers`); diagnostics reach the message collector through the pipeline's own
 * reporting. With [pre], the source session gets the IC "binaries from a previous compilation" symbol provider.
 */
class WarmFrontendPhase(
    private val st: WarmState,
    private val pre: Precompiled?,
    private val t: Timings,
    private val afterResolve: ((SingleModuleFrontendOutput) -> Unit)? = null,
) : PipelinePhase<ConfigurationPipelineArtifact, JvmFrontendPipelineArtifact>(
    name = "SpikeWarmFrontend",
    postActions = setOf(PerformanceNotifications.AnalysisFinished, CheckCompilationErrors.CheckDiagnosticCollector),
) {
    override fun executePhase(input: ConfigurationPipelineArtifact): JvmFrontendPipelineArtifact? {
        val configuration = input.configuration
        var t0 = System.nanoTime()
        var registeredFor: CompilerConfiguration? = null
        if (st.pinLvs && st.reuseLibs) {
            val pinned = st.lvs
            if (pinned == null || pinned.toString() != configuration.languageVersionSettings.toString()) {
                st.lvs = configuration.languageVersionSettings; st.libraryList = null; st.heldConfigurations += configuration
            } else {
                configuration.languageVersionSettings = pinned
            }
        }
        val env = st.env.takeIf { st.reuseEnv } ?: JvmFrontendPipelinePhase.createProjectEnvironment(
            configuration, if (st.reuseEnv) st.disposable else input.rootDisposable, EnvironmentConfigFiles.JVM_CONFIG_FILES,
        ).also { st.env = it; st.envBuilds++; st.libraryList = null; registeredFor = configuration; if (st.reuseEnv) st.heldConfigurations += configuration }
        if (registeredFor !== configuration) {
            // KotlinCoreEnvironment.ProjectEnvironment.registerExtensionsFromPlugins runs the plugin registrars into the
            // configuration's extension storage only when the environment is created; a reused environment must do it
            // for each new configuration or the compile has no plugin extensions (no krit checkers) at all.
            val storage = configuration.extensionsStorage!!
            for (registrar in configuration.getList(CompilerPluginRegistrar.COMPILER_PLUGIN_REGISTRARS)) {
                with(registrar) { storage.registerExtensions(configuration) }
            }
        }
        val sources = collectSources(configuration, env)
        val all = sources.allFiles
        t.env = System.nanoTime() - t0

        t0 = System.nanoTime()
        val moduleName = configuration.moduleName!!
        val rootName = Name.special("<$moduleName>")
        val lvs = configuration.languageVersionSettings
        val snapScope: AbstractProjectFileSearchScope? = (pre?.dir ?: st.snapshotDir)?.let { env.getSearchScopeByDirectories(setOf(it)) }
        val preScope: AbstractProjectFileSearchScope? = if (pre != null) {
            // Virtual exclusion: the snapshot directory never changes while a reused environment indexes it (the CLI
            // Java file manager caches class lookups, including misses), so stale outputs are subtracted from the scope.
            val ex = pre.excluded()
            if (ex.isEmpty()) snapScope else snapScope!! - env.getSearchScopeByIoFiles(ex, allowOutOfProjectRoots = true)
        } else null
        if (st.libraryList == null || !st.reuseLibs) {
            val libraryList = JvmFrontendPipelinePhase.createLibraryListForJvm(moduleName, configuration, emptyList())
            val librariesScope = env.getSearchScopeForProjectLibraries().let { if (snapScope != null) it - snapScope else it }
            val context = FirJvmSessionFactory.Context(configuration, env, librariesScope)
            val registrars = configuration.getCompilerExtensions(FirExtensionRegistrar)
            val shared = FirJvmSessionFactory.createSharedLibrarySession(rootName, registrars, lvs, context)
            FirJvmSessionFactory.createLibrarySession(shared, libraryList.moduleDataProvider, registrars, lvs, context)
            st.libraryList = libraryList; st.context = context; st.shared = shared; st.registrars = registrars; st.libBuilds++
        }
        t.libs = System.nanoTime() - t0

        t0 = System.nanoTime()
        val libraryList = st.libraryList!!
        val shared = st.shared!!
        val registrars = configuration.getCompilerExtensions(FirExtensionRegistrar)
        val moduleData = FirSourceModuleData(
            rootName, libraryList.regularDependencies, libraryList.dependsOnDependencies,
            libraryList.friendDependencies, JvmPlatforms.unspecifiedJvmPlatform,
        )
        if (st.freshJavaComponents && st.reuseLibs) {
            val ctx = st.context!!
            val f = FirJvmSessionFactory.Context::class.java.getDeclaredField("predefinedJavaComponents")
            f.isAccessible = true
            f.set(ctx, org.jetbrains.kotlin.fir.session.FirSharableJavaComponents(org.jetbrains.kotlin.fir.session.firCachesFactoryForCliMode))
        }
        val icContext = if (pre != null && preScope != null) {
            val ppp = org.jetbrains.kotlin.load.kotlin.incremental.IncrementalPackagePartProvider(lvs, listOf(SnapshotCache(pre)))
            IncrementalCompilationContext(ppp, preScope)
        } else null
        val session = FirJvmSessionFactory.createSourceSession(
            moduleData,
            env.getSearchScopeForProjectJavaSources(),
            { s -> icContext?.createSymbolProviders(s, moduleData, env) },
            registrars,
            configuration,
            st.context!!,
            needRegisterJavaElementFinder = false,
            kmpModuleKind = KmpModuleKind.SingleModule,
        ) {
            registerComponent(FirBuiltinSyntheticFunctionInterfaceProvider::class, shared.syntheticFunctionInterfacesSymbolProvider)
            useCheckers(CliOnlyLanguageVersionSettingsCheckers)
        }
        t.session = System.nanoTime() - t0

        t0 = System.nanoTime()
        val diagnostics = configuration.diagnosticsCollector
        val raw = session.buildFirViaLightTree(all, diagnostics, null)
        t.raw = System.nanoTime() - t0
        t0 = System.nanoTime()
        val out = resolveAndCheckFir(session, raw, diagnostics)
        listOf(out).runPlatformCheckers(diagnostics)
        t.resolveCheck = System.nanoTime() - t0
        if (System.getenv("SPIKE_DEBUG") != null) {
            System.err.println("debug: diagnostics=${diagnostics.diagnostics.size} byFactory=${diagnostics.diagnostics.groupingBy { it.factoryName }.eachCount()} registrars=${registrars.map { it.javaClass.name }} rules=${dev.jasonpearson.krit.fir.FirRuleDiscovery.enabled().size}")
        }
        afterResolve?.invoke(out)
        return JvmFrontendPipelineArtifact(AllModulesFrontendOutput(listOf(out)), configuration, env, all)
    }
}

/** IncrementalCache over a snapshot directory: only the module mapping is consulted by the frontend. */
class SnapshotCache(private val pre: Precompiled) : org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCache {
    override fun getObsoletePackageParts(): Collection<String> = emptyList()
    override fun getObsoleteMultifileClasses(): Collection<String> = emptyList()
    override fun getStableMultifileFacadeParts(facadeInternalName: String): Collection<String>? = null
    override fun getPackagePartData(partInternalName: String) = null
    override fun getModuleMappingData(): ByteArray? = pre.moduleMapping()
    override fun getClassFilePath(internalClassName: String): String = File(pre.dir, "$internalClassName.class").path
    override fun close() {}
}

typealias Phase = CompilerPhase<PipelineContext, ArgumentsPipelineArtifact<K2JVMCompilerArguments>, *>

object Phases {
    fun warmFrontend(st: WarmState, pre: Precompiled?, t: Timings, after: ((SingleModuleFrontendOutput) -> Unit)? = null): Phase =
        JvmConfigurationPipelinePhase then WarmFrontendPhase(st, pre, t, after)

    fun warmFull(st: WarmState, pre: Precompiled?, t: Timings, after: ((SingleModuleFrontendOutput) -> Unit)? = null): Phase =
        JvmConfigurationPipelinePhase then WarmFrontendPhase(st, pre, t, after) then
            JvmFir2IrPipelinePhase then JvmBackendPipelinePhase then JvmWriteOutputsPhase
}

/** Runs [phase] for [args] under a krit check context over [requested]; [services] carries IC trackers/components. */
fun runCheck(
    phase: Phase, args: K2JVMCompilerArguments, requested: Collection<String>,
    services: Services = Services.EMPTY, rules: Set<String> = emptySet(),
): CheckOut {
    val k = KritCheck(requested, rules)
    val exit = k.run { mc -> SpikePipeline(phase).execute(args, services, mc) }
    return k.out(exit)
}

/** The shipped compile paths, for baselines: frontend-only (`FrontendOnlyJvmPipeline`) and the full `K2JVMCompiler`. */
fun runShipped(
    args: K2JVMCompilerArguments, requested: Collection<String>, frontendOnly: Boolean,
    services: Services = Services.EMPTY, rules: Set<String> = emptySet(),
): CheckOut {
    val k = KritCheck(requested, rules)
    val exit = k.run { mc ->
        if (frontendOnly) dev.jasonpearson.krit.fir.runner.FrontendOnlyJvmPipeline().execute(args, services, mc)
        else org.jetbrains.kotlin.cli.jvm.K2JVMCompiler().exec(mc, services, args)
    }
    return k.out(exit)
}

@Suppress("unused")
private fun unusedImports(c: CompilerConfiguration, a: PipelineArtifact) = Unit
