package dev.krit.gradle

import org.gradle.api.GradleException
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.FileTree
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.plugins.BasePlugin
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.IgnoreEmptyDirectories
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.Optional
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.SkipWhenEmpty
import org.gradle.api.tasks.SourceTask
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import java.io.File
import javax.inject.Inject

/**
 * Gradle task that invokes the krit binary to run static analysis on Kotlin sources.
 *
 * Supports multiple report formats (SARIF, JSON, plain text, Checkstyle) configured
 * via the reports DSL. Fails the build if findings are detected (unless ignoreFailures
 * is set).
 */
@CacheableTask
abstract class KritCheckTask @Inject constructor(
    private val execOps: ExecOperations,
) : SourceTask() {

    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val kritBinary: RegularFileProperty

    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val config: RegularFileProperty

    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val baseline: RegularFileProperty

    @get:Input
    abstract val allRules: Property<Boolean>

    @get:Input
    abstract val ignoreFailures: Property<Boolean>

    @get:Input
    abstract val parallel: Property<Int>

    @get:Input
    abstract val noCache: Property<Boolean>

    @get:Input
    abstract val typeInference: Property<Boolean>

    @get:Internal
    abstract val cacheDir: DirectoryProperty

    // --- Report configuration properties ---

    @get:Input
    abstract val sarifRequired: Property<Boolean>

    @get:OutputFile
    @get:Optional
    abstract val sarifOutput: RegularFileProperty

    @get:Input
    abstract val jsonRequired: Property<Boolean>

    @get:OutputFile
    @get:Optional
    abstract val jsonOutput: RegularFileProperty

    @get:Input
    abstract val plainRequired: Property<Boolean>

    @get:OutputFile
    @get:Optional
    abstract val plainOutput: RegularFileProperty

    @get:Input
    abstract val checkstyleRequired: Property<Boolean>

    @get:OutputFile
    @get:Optional
    abstract val checkstyleOutput: RegularFileProperty

    /**
     * Legacy property kept for backward compatibility. If set, it overrides the
     * SARIF output location from the reports DSL.
     */
    @get:OutputFile
    @get:Optional
    abstract val sarifReport: RegularFileProperty

    init {
        group = BasePlugin.BUILD_GROUP
        description = "Run krit static analysis on Kotlin sources"
    }

    @InputFiles
    @SkipWhenEmpty
    @IgnoreEmptyDirectories
    @PathSensitive(PathSensitivity.RELATIVE)
    override fun getSource(): FileTree = super.getSource()

    @TaskAction
    fun check() {
        // Collect enabled reports as (format, outputFile) pairs
        val enabledReports = buildEnabledReports()

        if (enabledReports.isEmpty()) {
            // Fall back to SARIF if nothing is enabled (preserves old behavior)
            val fallback = sarifReport.orNull?.asFile
                ?: project.layout.buildDirectory.file("reports/krit/krit.sarif").get().asFile
            enabledReports.add("sarif" to fallback)
        }

        // Ensure parent directories exist for all report files
        enabledReports.forEach { (_, file) -> file.parentFile.mkdirs() }

        // Use the first enabled report as the primary output format
        val primaryReport = enabledReports.first()

        val args = buildList {
            add("--format=${primaryReport.first}")
            add("-o"); add(primaryReport.second.absolutePath)
            add("-j"); add(parallel.get().toString())
            if (allRules.get()) add("--all-rules")
            if (config.isPresent) { add("--config"); add(config.get().asFile.absolutePath) }
            if (baseline.isPresent) { add("--baseline"); add(baseline.get().asFile.absolutePath) }
            if (noCache.get()) add("--no-cache")
            if (!typeInference.get()) add("--no-type-inference")
            if (cacheDir.isPresent) { add("--cache-dir"); add(cacheDir.get().asFile.absolutePath) }
            add("-q")
            source.files.forEach { add(it.absolutePath) }
        }

        val result = execOps.exec { spec ->
            spec.executable = kritBinary.get().asFile.absolutePath
            spec.args = args
            spec.isIgnoreExitValue = true
        }

        // Run additional report formats if more than one is enabled
        for (i in 1 until enabledReports.size) {
            val (format, outputFile) = enabledReports[i]
            val extraArgs = buildList {
                add("--format=$format")
                add("-o"); add(outputFile.absolutePath)
                add("-j"); add(parallel.get().toString())
                if (allRules.get()) add("--all-rules")
                if (config.isPresent) { add("--config"); add(config.get().asFile.absolutePath) }
                if (baseline.isPresent) { add("--baseline"); add(baseline.get().asFile.absolutePath) }
                if (noCache.get()) add("--no-cache")
                if (!typeInference.get()) add("--no-type-inference")
                if (cacheDir.isPresent) { add("--cache-dir"); add(cacheDir.get().asFile.absolutePath) }
                add("-q")
                source.files.forEach { add(it.absolutePath) }
            }

            execOps.exec { spec ->
                spec.executable = kritBinary.get().asFile.absolutePath
                spec.args = extraArgs
                spec.isIgnoreExitValue = true
            }
        }

        if (result.exitValue != 0 && !ignoreFailures.get()) {
            val reportPaths = enabledReports.joinToString(", ") { it.second.absolutePath }
            throw GradleException(
                "krit analysis found issues (exit code ${result.exitValue}). " +
                    "See reports at: $reportPaths"
            )
        }
    }

    private fun buildEnabledReports(): MutableList<Pair<String, File>> {
        val reports = mutableListOf<Pair<String, File>>()

        // Legacy sarifReport property takes precedence for SARIF
        if (sarifReport.isPresent) {
            reports.add("sarif" to sarifReport.get().asFile)
        } else if (sarifRequired.getOrElse(false)) {
            sarifOutput.orNull?.asFile?.let { reports.add("sarif" to it) }
        }

        if (jsonRequired.getOrElse(false)) {
            jsonOutput.orNull?.asFile?.let { reports.add("json" to it) }
        }

        if (plainRequired.getOrElse(false)) {
            plainOutput.orNull?.asFile?.let { reports.add("plain" to it) }
        }

        if (checkstyleRequired.getOrElse(false)) {
            checkstyleOutput.orNull?.asFile?.let { reports.add("checkstyle" to it) }
        }

        return reports
    }
}
