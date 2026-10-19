@file:OptIn(org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class, org.jetbrains.kotlin.config.CompilerConfiguration.Internals::class, org.jetbrains.kotlin.config.MessageCollectorAccess::class)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.isIsolatable
import org.jetbrains.kotlin.backend.common.phaser.then
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.diagnosticsCollector
import org.jetbrains.kotlin.cli.common.fir.FirDiagnosticsCompilerResultsReporter
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.config.messageCollector
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.GroupingMessageCollector
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.pipeline.AbstractCliPipeline
import org.jetbrains.kotlin.cli.pipeline.ArgumentsPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PipelineContext
import org.jetbrains.kotlin.cli.pipeline.PipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmBackendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmConfigurationPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFir2IrPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmWriteOutputsPhase
import org.jetbrains.kotlin.config.Services
import org.jetbrains.kotlin.config.phaser.CompilerPhase
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.util.PerformanceManager
import org.jetbrains.kotlin.util.PerformanceManagerImpl
import java.util.concurrent.ConcurrentLinkedQueue

/**
 * Passes the frontend's output on and tells [onDone] with the compilation's message collector. It runs only when
 * the frontend finished without a compiler error: the phase before it stops the pipeline otherwise.
 */
private class FrontendDonePhase(private val onDone: (CompilerConfiguration) -> Boolean) :
    PipelinePhase<JvmFrontendPipelineArtifact, JvmFrontendPipelineArtifact>(name = "KritFrontendDone") {
    /** Null ends the pipeline here: [onDone] says the backend must not run. */
    override fun executePhase(input: JvmFrontendPipelineArtifact): JvmFrontendPipelineArtifact? =
        if (onDone(input.configuration)) input else null
}

/**
 * The stock K2 JVM phases (configuration, frontend, fir2ir, backend, output), with a marker after the frontend.
 * The frontend is the same phase [dev.jasonpearson.krit.fir.runner.FrontendOnlyJvmPipeline] runs, so everything up
 * to the marker reports exactly what a frontend-only check reports.
 *
 * Two phases of the compiler's own JVM chain are left out. `FrontendFilesForPluginsGenerationPipelinePhase` lets
 * compiler plugins generate source files for the frontend; krit's plugin generates none, and the frontend-only
 * pipeline that is today's check does not run it either. `JvmSerializeCommonMetadataPipelinePhase` writes common
 * metadata for multiplatform compilations, which never get a snapshot (`expect`/`actual` falls back).
 */
private class SnapshotJvmPipeline(private val frontendDone: (CompilerConfiguration) -> Boolean) : AbstractCliPipeline<K2JVMCompilerArguments>() {
    override val defaultPerformanceManager: PerformanceManager =
        PerformanceManagerImpl(JvmPlatforms.defaultJvmPlatform, "Kotlin to JVM Compiler (snapshot)")

    override fun createCompoundPhase(
        arguments: K2JVMCompilerArguments,
    ): CompilerPhase<PipelineContext, ArgumentsPipelineArtifact<K2JVMCompilerArguments>, *> =
        JvmConfigurationPipelinePhase then JvmFrontendPipelinePhase then FrontendDonePhase(frontendDone) then
            JvmFir2IrPipelinePhase then JvmBackendPipelinePhase then JvmWriteOutputsPhase
}

/**
 * One whole-module check compile that also writes the module's binary snapshot: the stock phases with the backend,
 * `-Xreport-output-files`, into [snapshot]'s directory.
 *
 * It is a check first. The messages and the lookup recording see the frontend exactly as in a frontend-only compile
 * and nothing after it: once the frontend is clean, [run] hands over every message the frontend produced, then drops
 * every later one (a backend diagnostic must not turn a clean verdict into an error) and freezes the lookups. The
 * compile's verdicts are therefore the verdicts of a frontend-only check, whatever the backend does. A module with a
 * compiler error never reaches the backend, so it gets no snapshot. A backend failure after a clean frontend only
 * costs the snapshot.
 *
 * The compiler keeps the frontend's diagnostics (every krit finding is a warning) in its diagnostics collector and
 * hands them to the message collector only when the pipeline ends, which here is after the backend. The marker
 * therefore reports and flushes them itself, before the backend runs; the end-of-pipeline repeat is dropped.
 */
internal class SnapshotBuild(val snapshot: Snapshot, private val lookups: CompileLookups?) {
    private val outputMessages = ConcurrentLinkedQueue<String>()

    /** The frontend finished without a compiler error, so the backend ran (or failed trying). */
    @Volatile var frontendClean = false
        private set

    /** Why the backend run failed after a clean frontend; null when it did not. */
    @Volatile var failure: Throwable? = null
        private set

    /** True when [snapshot] now holds the module's class files. */
    @Volatile var built = false
        private set

    // Messages pass through until the frontend's are delivered; after that they are the backend's and are dropped.
    @Volatile private var dropping = false

    fun run(args: K2JVMCompilerArguments, services: Services, messages: MessageCollector): ExitCode {
        val gate = object : MessageCollector {
            override fun clear() = messages.clear()
            override fun hasErrors() = messages.hasErrors()
            override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) {
                if (severity == CompilerMessageSeverity.OUTPUT) { outputMessages += message; return }
                if (!dropping) messages.report(severity, message, location)
            }
        }
        val pipeline = SnapshotJvmPipeline { configuration ->
            // Without the buffer to flush the frontend's messages cannot be told from the backend's: fail closed,
            // no backend and no snapshot, instead of letting backend messages reach the verdict.
            val buffer = (configuration.messageCollector as? GroupingMessageCollector)
                ?.takeUnless { IncrementalTestHook.hideMessageBuffer }
            if (buffer == null) {
                failure = IllegalStateException("the compiler's message buffer is not available; no snapshot")
                return@SnapshotJvmPipeline false
            }
            frontendClean = true
            FirDiagnosticsCompilerResultsReporter.reportToMessageCollector(configuration.diagnosticsCollector, configuration)
            buffer.flush()
            dropping = true
            lookups?.freeze()
            IncrementalTestHook.onStage?.invoke(IncrementalStage.SNAPSHOT_BACKEND)
            true
        }
        val exit = try {
            pipeline.execute(args, services, gate)
        } catch (t: Throwable) {
            // Before the marker this is today's failure and propagates as before.
            if (!frontendClean || !isIsolatable(t)) throw t
            failure = t
            ExitCode.OK
        }
        if (!frontendClean) {
            // Ended at the marker without a backend: the frontend's messages went out normally at the end.
            return if (failure != null) ExitCode.OK else exit
        }
        if (failure == null && exit != ExitCode.OK) failure = IllegalStateException("snapshot backend exited with $exit")
        if (failure == null) {
            try {
                snapshot.adopt(Snapshot.parseOutputs(outputMessages))
                built = true
            } catch (t: Throwable) {
                if (!isIsolatable(t)) throw t
                failure = t
            }
        }
        // The frontend was clean, so as far as the check goes the compile succeeded.
        return ExitCode.OK
    }
}
