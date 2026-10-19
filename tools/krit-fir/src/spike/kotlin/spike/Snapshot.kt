package spike

import org.jetbrains.kotlin.cli.jvm.compiler.VfsBasedProjectEnvironment
import org.jetbrains.kotlin.cli.jvm.compiler.legacy.pipeline.IncrementalCompilationComponentsWithCustomScope
import org.jetbrains.kotlin.cli.pipeline.CheckCompilationErrors
import org.jetbrains.kotlin.cli.pipeline.ConfigurationPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PerformanceNotifications
import org.jetbrains.kotlin.cli.pipeline.PipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmBackendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmConfigurationPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFir2IrPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmWriteOutputsPhase
import org.jetbrains.kotlin.backend.common.phaser.then
import org.jetbrains.kotlin.config.Services
import org.jetbrains.kotlin.fir.pipeline.SingleModuleFrontendOutput
import org.jetbrains.kotlin.fir.session.environment.AbstractProjectFileSearchScope
import org.jetbrains.kotlin.incremental.components.ImportTracker
import org.jetbrains.kotlin.incremental.components.LookupTracker
import org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCache
import org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCompilationComponents
import org.jetbrains.kotlin.metadata.jvm.JvmModuleProtoBuf
import org.jetbrains.kotlin.metadata.deserialization.MetadataVersion
import org.jetbrains.kotlin.metadata.jvm.deserialization.ModuleMapping
import org.jetbrains.kotlin.metadata.jvm.deserialization.PackageParts
import org.jetbrains.kotlin.metadata.jvm.deserialization.serializeToByteArray
import org.jetbrains.kotlin.modules.TargetId
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption

/**
 * The module's binary snapshot: class files from a full compile with the backend, the source -> output map (from the
 * compiler's `-Xreport-output-files` OUTPUT messages) and the module mapping (`META-INF/<module>.kotlin_module`).
 */
class Snapshot(val dir: File, val moduleName: String) {
    val outputs = HashMap<String, List<String>>() // canonical source -> paths relative to dir
    private val stash = File(dir.parentFile, dir.name + "-aside").also { it.mkdirs() }
    private val parked = HashMap<String, List<String>>() // moved aside for the current compile
    var classFiles = 0
        private set

    fun mappingFile() = File(dir, "META-INF/$moduleName.kotlin_module")
    fun mappingBytes(): ByteArray? = mappingFile().takeIf { it.isFile }?.readBytes()
    /** Virtual mode: never touch the directory; park/drop only record files to subtract from the IC scope. */
    var virtual = false
    private val virtualParked = HashSet<String>()
    private val virtualDropped = HashSet<String>()
    val precompiled = Precompiled(dir, { (virtualParked + virtualDropped).map { File(dir, it) } }) { mappingBytes() }

    /** Takes over a full compile's output directory [out] (already == [dir]) and its OUTPUT map. */
    fun adopt(outs: Map<String, List<String>>) {
        outputs.clear()
        for ((src, ps) in outs) outputs[src] = ps.filter { it.endsWith(".class") }.map { File(it).relativeTo(dir).path }
        classFiles = outputs.values.sumOf { it.size }
    }

    /** Moves the outputs of [sources] out of the snapshot for the duration of a slice compile. */
    fun park(sources: Collection<String>) {
        if (virtual) { for (s in sources) outputs[s]?.let { virtualParked += it; parked[s] = it }; return }
        for (s in sources) {
            val rels = outputs[s] ?: continue
            for (r in rels) move(File(dir, r), File(stash, r))
            parked[s] = rels
        }
    }

    /** Puts parked outputs back, except for [drop] (sources whose outputs are stale for good). */
    fun unpark(drop: Set<String> = emptySet()) {
        if (virtual) {
            for ((s, rels) in parked) if (s in drop) { virtualDropped += rels; outputs.remove(s) }
            virtualParked.clear(); parked.clear()
            classFiles = outputs.values.sumOf { it.size }
            return
        }
        for ((s, rels) in parked) {
            if (s in drop) { rels.forEach { File(stash, it).delete() }; outputs.remove(s); continue }
            for (r in rels) move(File(stash, r), File(dir, r))
        }
        parked.clear()
        classFiles = outputs.values.sumOf { it.size }
    }

    /** Removes [sources]' outputs for good (deleted files, or option (b): sources kept in every slice). */
    fun drop(sources: Collection<String>) {
        if (virtual) { for (s in sources) outputs.remove(s)?.let { virtualDropped += it }; classFiles = outputs.values.sumOf { it.size }; return }
        for (s in sources) outputs.remove(s)?.forEach { File(dir, it).delete() }
        classFiles = outputs.values.sumOf { it.size }
    }

    /**
     * Option (a): a slice compile with the backend wrote [outs] (absolute paths under [tmp]) for the parked [compiled]
     * sources. Replace their outputs in the snapshot and merge the module mapping.
     */
    fun patch(tmp: File, outs: Map<String, List<String>>, compiled: Collection<String>) {
        val oldInternal = HashSet<String>()
        for (s in compiled) {
            parked.remove(s)?.forEach { r -> oldInternal += r.removeSuffix(".class"); File(stash, r).delete() }
            outputs.remove(s)?.forEach { r -> oldInternal += r.removeSuffix(".class"); File(dir, r).delete() }
        }
        for ((src, ps) in outs) {
            val rels = ArrayList<String>()
            for (p in ps) {
                val rel = File(p).relativeTo(tmp).path
                if (!rel.endsWith(".class")) continue
                move(File(p), File(dir, rel)); rels += rel
            }
            outputs[src] = rels
        }
        val merged = LinkedHashMap<String, PackageParts>()
        load(mappingBytes())?.packageFqName2Parts?.forEach { (pkg, parts) ->
            val copy = PackageParts(pkg)
            for (p in parts.parts) if (p !in oldInternal) copy.addPart(p, parts.getMultifileFacadeName(p))
            merged[pkg] = copy
        }
        load(File(tmp, "META-INF/$moduleName.kotlin_module").takeIf { it.isFile }?.readBytes())?.packageFqName2Parts?.forEach { (pkg, parts) ->
            val into = merged.getOrPut(pkg) { PackageParts(pkg) }
            for (p in parts.parts) into.addPart(p, parts.getMultifileFacadeName(p))
        }
        val b = JvmModuleProtoBuf.Module.newBuilder()
        for (pp in merged.values) pp.addTo(b)
        mappingFile().parentFile.mkdirs()
        mappingFile().writeBytes(b.build().serializeToByteArray(MetadataVersion.INSTANCE, 0))
        classFiles = outputs.values.sumOf { it.size }
    }

    private fun load(bytes: ByteArray?): ModuleMapping? = bytes?.let {
        ModuleMapping.loadModuleMapping(it, "snapshot", skipMetadataVersionCheck = true, isJvmPackageNameSupported = true) {}
    }

    private fun move(from: File, to: File) {
        if (!from.exists()) return
        to.parentFile.mkdirs()
        Files.move(from.toPath(), to.toPath(), StandardCopyOption.REPLACE_EXISTING)
    }
}

/**
 * The IC components the stock CLI frontend reads from `Services` (`JvmConfigurationUpdater.setupIncrementalCompilationServices`,
 * then `prepareIncrementalCompilationContextAndLibrariesScope`): the snapshot directory becomes the "binaries from a
 * previous compilation" scope of the *same* module, and is subtracted from the libraries scope.
 */
class SnapshotComponents(private val snap: Snapshot) : IncrementalCompilationComponentsWithCustomScope {
    override fun getIncrementalCache(target: TargetId): IncrementalCache = SnapshotCache(snap.precompiled)
    override fun createSearchScope(projectEnvironment: VfsBasedProjectEnvironment): AbstractProjectFileSearchScope =
        projectEnvironment.getSearchScopeByDirectories(setOf(snap.dir))
}

fun services(lookups: CompileLookups?, snap: Snapshot?): Services {
    val b = Services.Builder()
    if (lookups != null) {
        b.register(LookupTracker::class.java, lookups)
        b.register(ImportTracker::class.java, lookups)
    }
    if (snap != null) b.register(IncrementalCompilationComponents::class.java, SnapshotComponents(snap))
    return b.build()
}

/** The stock `JvmFrontendPipelinePhase` plus a read-only hook over its FIR output (surfaces), before the error check. */
class StockFrontendWithHook(private val after: ((SingleModuleFrontendOutput) -> Unit)?) :
    PipelinePhase<ConfigurationPipelineArtifact, JvmFrontendPipelineArtifact>(
        name = "StockFrontendWithHook",
        postActions = setOf(PerformanceNotifications.AnalysisFinished, CheckCompilationErrors.CheckDiagnosticCollector),
    ) {
    override fun executePhase(input: ConfigurationPipelineArtifact): JvmFrontendPipelineArtifact? =
        JvmFrontendPipelinePhase.executePhase(input)?.also { a -> after?.let { h -> a.frontendOutput.outputs.forEach(h) } }
}

object StockPhases {
    fun frontend(after: ((SingleModuleFrontendOutput) -> Unit)?): Phase = JvmConfigurationPipelinePhase then StockFrontendWithHook(after)
    fun full(after: ((SingleModuleFrontendOutput) -> Unit)?): Phase =
        JvmConfigurationPipelinePhase then StockFrontendWithHook(after) then JvmFir2IrPipelinePhase then JvmBackendPipelinePhase then JvmWriteOutputsPhase
}
