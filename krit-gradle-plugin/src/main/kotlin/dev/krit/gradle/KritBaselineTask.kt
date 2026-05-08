package dev.krit.gradle

import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.plugins.BasePlugin
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Optional
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import javax.inject.Inject

/**
 * Gradle task that invokes the krit binary to create a baseline file.
 *
 * A baseline captures all current findings so that subsequent runs only report
 * new issues. This is useful when adopting krit on an existing codebase.
 */
abstract class KritBaselineTask @Inject constructor(
    private val execOps: ExecOperations,
) : DefaultTask() {

    @get:InputFiles
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val source: ConfigurableFileCollection

    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val kritBinary: RegularFileProperty

    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val config: RegularFileProperty

    @get:OutputFile
    abstract val baselineFile: RegularFileProperty

    @get:Input
    abstract val allRules: Property<Boolean>

    @get:Input
    abstract val parallel: Property<Int>

    @get:Input
    abstract val noCache: Property<Boolean>

    @get:Input
    abstract val typeInference: Property<Boolean>

    init {
        group = BasePlugin.BUILD_GROUP
        description = "Create a krit baseline file from current findings"
    }

    @TaskAction
    fun createBaseline() {
        val sourceFiles = source.files
        if (sourceFiles.isEmpty()) {
            logger.info("No source files found, skipping baseline creation")
            return
        }

        val outputFile = baselineFile.get().asFile
        outputFile.parentFile.mkdirs()

        val args = buildList {
            add("--create-baseline")
            add("--baseline"); add(outputFile.absolutePath)
            add("-j"); add(parallel.get().toString())
            if (allRules.get()) add("--all-rules")
            if (config.isPresent) { add("--config"); add(config.get().asFile.absolutePath) }
            if (noCache.get()) add("--no-cache")
            if (!typeInference.get()) add("--no-type-inference")
            add("-q")
            sourceFiles.forEach { add(it.absolutePath) }
        }

        val result = execOps.exec { spec ->
            spec.executable = kritBinary.get().asFile.absolutePath
            spec.args = args
            spec.isIgnoreExitValue = true
        }

        if (result.exitValue != 0) {
            throw GradleException(
                "krit baseline creation failed (exit code ${result.exitValue})"
            )
        }

        logger.lifecycle("krit baseline created at ${outputFile.absolutePath}")
    }
}
