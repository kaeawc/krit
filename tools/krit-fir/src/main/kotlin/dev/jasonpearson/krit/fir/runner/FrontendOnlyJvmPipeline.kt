package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.backend.common.phaser.then
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.pipeline.AbstractCliPipeline
import org.jetbrains.kotlin.cli.pipeline.ArgumentsPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PipelineContext
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmConfigurationPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelinePhase
import org.jetbrains.kotlin.config.phaser.CompilerPhase
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.util.PerformanceManager
import org.jetbrains.kotlin.util.PerformanceManagerImpl

/**
 * The first two phases of the compiler's `JvmCliPipeline`: configuration
 * (which also loads compiler plugins such as krit's own registrar) and the
 * K2 frontend, where FIR resolution, every FIR checker and diagnostic
 * reporting happen. The frontend phase's error check still turns compile
 * errors into [org.jetbrains.kotlin.cli.common.ExitCode.COMPILATION_ERROR].
 * fir2ir, the JVM backend and output writing are never run.
 */
internal class FrontendOnlyJvmPipeline : AbstractCliPipeline<K2JVMCompilerArguments>() {
    override val defaultPerformanceManager: PerformanceManager =
        PerformanceManagerImpl(JvmPlatforms.defaultJvmPlatform, "Kotlin to JVM Compiler (frontend only)")

    override fun createCompoundPhase(
        arguments: K2JVMCompilerArguments,
    ): CompilerPhase<PipelineContext, ArgumentsPipelineArtifact<K2JVMCompilerArguments>, *> =
        JvmConfigurationPipelinePhase then JvmFrontendPipelinePhase
}
