@file:OptIn(org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.FirRuleErrors
import dev.jasonpearson.krit.fir.runner.BundledStdlib
import dev.jasonpearson.krit.fir.runner.FindingCollector
import dev.jasonpearson.krit.fir.runner.FrontendOnlyJvmPipeline
import dev.jasonpearson.krit.fir.runner.KotlinCompilerSettings
import dev.jasonpearson.krit.fir.runner.resolveJvmTarget
import org.jetbrains.kotlin.backend.common.phaser.then
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.common.messages.OutputMessageUtil
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.cli.jvm.compiler.VfsBasedProjectEnvironment
import org.jetbrains.kotlin.cli.jvm.compiler.legacy.pipeline.IncrementalCompilationComponentsWithCustomScope
import org.jetbrains.kotlin.cli.pipeline.AbstractCliPipeline
import org.jetbrains.kotlin.cli.pipeline.ArgumentsPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PipelineContext
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmBackendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmConfigurationPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFir2IrPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmWriteOutputsPhase
import org.jetbrains.kotlin.config.Services
import org.jetbrains.kotlin.config.phaser.CompilerPhase
import org.jetbrains.kotlin.fir.session.environment.AbstractProjectFileSearchScope
import org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCache
import org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCompilationComponents
import org.jetbrains.kotlin.modules.TargetId
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.util.PerformanceManager
import org.jetbrains.kotlin.util.PerformanceManagerImpl
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption

// Test-only plumbing for a "slice compile against a binary snapshot": the dirty
// files are compiled from source and the rest of the module is read from class
// files of an earlier compile, through the compiler's own incremental-compilation
// hooks on the stock CLI pipeline. The production version belongs to the
// incremental FIR work (#865); keep this in test sources until then.

private val pluginJar: String =
    System.getProperty("krit.fir.plugin.jar") ?: error("-Dkrit.fir.plugin.jar is required")

internal fun canon(p: String): String = try { File(p).canonicalPath } catch (_: Exception) { File(p).absolutePath }

/**
 * Compiler arguments equal to `AnalysisSession.compilationArguments` for a non-module compile, plus an explicit
 * module name (a slice must share it with the snapshot so `internal` resolves) and the krit plugin.
 */
internal fun kritArgs(
    sources: List<String>,
    classpath: List<String>,
    out: File,
    moduleName: String = "main",
): K2JVMCompilerArguments = K2JVMCompilerArguments().apply {
    freeArgs = sources
    this.classpath = BundledStdlib.effectiveClasspath(classpath).joinToString(File.pathSeparator)
    jvmTarget = resolveJvmTarget("")
    KotlinCompilerSettings.parse(emptyList()).applyTo(this)
    destination = out.absolutePath
    noStdlib = true
    noReflect = true
    suppressWarnings = false
    reportAllWarnings = true
    pluginClasspaths = arrayOf(pluginJar)
    this.moduleName = moduleName
}

/** One file's verdict as Go consumes it, plus every compiler diagnostic. */
internal data class FileVerdict(
    val findings: List<String>,
    val error: Boolean,
    val ruleErrors: Map<String, String>,
    val diags: List<String>,
) {
    fun verdictKey() = Triple(findings, error, ruleErrors.keys.sorted())
}

internal class CheckOut(
    val verdicts: Map<String, FileVerdict>,
    val exit: ExitCode,
    val outputs: Map<String, List<String>>,
)

/** Collects every located compiler diagnostic per canonical file path, and the OUTPUT messages. */
private class AllDiags : MessageCollector {
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

private class Fanout(private val cs: List<MessageCollector>) : MessageCollector {
    override fun clear() = cs.forEach { it.clear() }
    override fun hasErrors() = cs.any { it.hasErrors() }
    override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) =
        cs.forEach { it.report(severity, message, location) }
}

/**
 * The krit side of one check compile, as `AnalysisSession.CheckPlan` sets it up: the rule context and the armed
 * rule-error recorder on the calling thread, a [FindingCollector] over the requested files.
 */
private class KritCheck(requested: Collection<String>) {
    private val requestedCanon: Map<String, String> = requested.associate { canon(it) to canon(it) }
    val findings = FindingCollector(requestedCanon)
    val all = AllDiags()
    private val recorder = FirRuleErrorRecorder(null)
    private val ctx = FirRuleCompileContext(files = requested.toCollection(LinkedHashSet()))
    private val collector: MessageCollector = Fanout(listOf(findings, all))

    fun run(body: (MessageCollector) -> ExitCode): CheckOut {
        FirRuleContext.begin(ctx)
        FirRuleErrors.begin(recorder)
        val exit = try {
            body(collector)
        } finally {
            FirRuleErrors.end()
            FirRuleContext.end()
        }
        return out(exit)
    }

    private fun out(exit: ExitCode): CheckOut {
        val ruleErrs = HashMap<String, MutableMap<String, String>>() // file -> rule -> message
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
        return CheckOut(verdicts, exit, parseOutputs(all.outputMessages))
    }

    private fun parseOutputs(msgs: List<String>): Map<String, List<String>> {
        val m = HashMap<String, MutableList<String>>()
        for (msg in msgs) {
            val o = OutputMessageUtil.parseOutputMessage(msg) ?: continue
            for (s in o.sourceFiles) m.getOrPut(canon(s.path)) { ArrayList() }.add(o.outputFile!!.path)
        }
        return m
    }
}

internal typealias Phase = CompilerPhase<PipelineContext, ArgumentsPipelineArtifact<K2JVMCompilerArguments>, *>

private class PhasePipeline(private val phase: Phase) : AbstractCliPipeline<K2JVMCompilerArguments>() {
    override val defaultPerformanceManager: PerformanceManager =
        PerformanceManagerImpl(JvmPlatforms.defaultJvmPlatform, "snapshot-compile")
    override fun createCompoundPhase(arguments: K2JVMCompilerArguments) = phase
}

internal object StockPhases {
    /** The stock frontend (parse, resolve, check): what a check compile of a slice runs. */
    val frontend: Phase get() = JvmConfigurationPipelinePhase then JvmFrontendPipelinePhase
    /** Frontend plus backend and class-file output: what produces a snapshot. */
    val full: Phase
        get() = JvmConfigurationPipelinePhase then JvmFrontendPipelinePhase then JvmFir2IrPipelinePhase then
            JvmBackendPipelinePhase then JvmWriteOutputsPhase
}

/** Runs [phase] for [args] under a krit check context over [requested]; [services] carries the IC components. */
internal fun runCheck(
    phase: Phase,
    args: K2JVMCompilerArguments,
    requested: Collection<String>,
    services: Services = Services.EMPTY,
): CheckOut = KritCheck(requested).run { mc -> PhasePipeline(phase).execute(args, services, mc) }

/** The shipped frontend-only path (`FrontendOnlyJvmPipeline`), or the full `K2JVMCompiler`, for baselines. */
internal fun runShipped(
    args: K2JVMCompilerArguments,
    requested: Collection<String>,
    frontendOnly: Boolean,
): CheckOut = KritCheck(requested).run { mc ->
    if (frontendOnly) FrontendOnlyJvmPipeline().execute(args, Services.EMPTY, mc)
    else K2JVMCompiler().exec(mc, Services.EMPTY, args)
}

/**
 * The module's binary snapshot: class files of a full compile (the source -> output map comes from the compiler's
 * `-Xreport-output-files` OUTPUT messages). [park] moves the outputs of the sources being compiled out of the
 * directory for the duration of a slice compile, [unpark] puts them back.
 */
internal class Snapshot(val dir: File) {
    private val outputs = HashMap<String, List<String>>() // canonical source -> paths relative to dir
    private val stash = File(dir.parentFile, dir.name + "-aside").also { it.mkdirs() }
    private val parked = HashMap<String, List<String>>()

    fun moduleMapping(moduleName: String = "main"): ByteArray? =
        File(dir, "META-INF/$moduleName.kotlin_module").takeIf { it.isFile }?.readBytes()

    fun adopt(outs: Map<String, List<String>>) {
        outputs.clear()
        for ((src, ps) in outs) outputs[src] = ps.filter { it.endsWith(".class") }.map { File(it).relativeTo(dir).path }
    }

    fun park(sources: Collection<String>) {
        for (s in sources) {
            val rels = outputs[s] ?: continue
            for (r in rels) move(File(dir, r), File(stash, r))
            parked[s] = rels
        }
    }

    fun unpark() {
        for ((_, rels) in parked) for (r in rels) move(File(stash, r), File(dir, r))
        parked.clear()
    }

    private fun move(from: File, to: File) {
        if (!from.exists()) return
        to.parentFile.mkdirs()
        Files.move(from.toPath(), to.toPath(), StandardCopyOption.REPLACE_EXISTING)
    }
}

/** Only the module mapping is consulted by the frontend. */
private class SnapshotCache(private val snap: Snapshot) : IncrementalCache {
    override fun getObsoletePackageParts(): Collection<String> = emptyList()
    override fun getObsoleteMultifileClasses(): Collection<String> = emptyList()
    override fun getStableMultifileFacadeParts(facadeInternalName: String): Collection<String>? = null
    override fun getPackagePartData(partInternalName: String) = null
    override fun getModuleMappingData(): ByteArray? = snap.moduleMapping()
    override fun getClassFilePath(internalClassName: String): String = File(snap.dir, "$internalClassName.class").path
    override fun close() {}
}

/**
 * The IC components the stock CLI frontend reads from `Services`: the snapshot directory becomes the "binaries from
 * a previous compilation" scope of the same module, and is subtracted from the libraries scope.
 */
private class SnapshotComponents(private val snap: Snapshot) : IncrementalCompilationComponentsWithCustomScope {
    override fun getIncrementalCache(target: TargetId): IncrementalCache = SnapshotCache(snap)
    override fun createSearchScope(projectEnvironment: VfsBasedProjectEnvironment): AbstractProjectFileSearchScope =
        projectEnvironment.getSearchScopeByDirectories(setOf(snap.dir))
}

internal fun snapshotServices(snap: Snapshot): Services =
    Services.Builder().register(IncrementalCompilationComponents::class.java, SnapshotComponents(snap)).build()
