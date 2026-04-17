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
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import javax.inject.Inject

/**
 * Gradle task that invokes the krit binary with --fix to apply auto-fixes.
 *
 * The fix level controls which categories of fixes are applied:
 * - cosmetic: whitespace, formatting, trailing commas
 * - idiomatic: Kotlin idiom improvements (default)
 * - semantic: fixes that may change runtime behavior
 */
abstract class KritFormatTask @Inject constructor(
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

    @get:Input
    @get:Optional
    abstract val fixLevel: Property<String>

    @get:Input
    abstract val parallel: Property<Int>

    @get:Input
    abstract val noCache: Property<Boolean>

    @get:Input
    abstract val typeInference: Property<Boolean>

    init {
        group = BasePlugin.BUILD_GROUP
        description = "Apply krit auto-fixes to Kotlin sources"
        // Format tasks are never cacheable -- they modify source files in place
        outputs.upToDateWhen { false }
    }

    @TaskAction
    fun format() {
        val sourceFiles = source.files
        if (sourceFiles.isEmpty()) {
            logger.info("No source files found, skipping krit format")
            return
        }

        val args = buildList {
            add("--fix")
            if (fixLevel.isPresent) {
                add("--fix-level")
                add(fixLevel.get())
            }
            add("-j"); add(parallel.get().toString())
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
                "krit format failed (exit code ${result.exitValue})"
            )
        }

        logger.lifecycle("krit format applied fixes to ${sourceFiles.size} source files")
    }
}
