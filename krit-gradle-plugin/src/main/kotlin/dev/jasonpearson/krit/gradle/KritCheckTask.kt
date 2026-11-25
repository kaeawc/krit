package dev.jasonpearson.krit.gradle

import org.gradle.api.GradleException
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.FileTree
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.plugins.BasePlugin
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.CompileClasspath
import org.gradle.api.tasks.Classpath
import org.gradle.api.tasks.IgnoreEmptyDirectories
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.Nested
import org.gradle.api.tasks.Optional
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.SkipWhenEmpty
import org.gradle.api.tasks.SourceTask
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import java.io.File
import java.io.ByteArrayOutputStream
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

    /**
     * An explicit krit executable (`advanced.binary`). When absent, the binary for
     * [toolVersion] is resolved, and downloaded if needed, only when the task runs,
     * so that neither task-graph calculation nor the configuration cache depends on it.
     */
    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val kritBinary: RegularFileProperty

    /** The krit release resolved when [kritBinary] is absent; empty when it is set. */
    @get:Input
    abstract val toolVersion: Property<String>

    @get:Internal
    abstract val binaryResolver: Property<KritBinaryResolver>

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

    // Neither changes the findings, and parallel defaults to the CPU count, so as
    // inputs they would keep machines with different CPUs from sharing cache entries.
    @get:Internal
    abstract val parallel: Property<Int>

    @get:Internal
    abstract val noCache: Property<Boolean>

    @get:Input
    abstract val typeInference: Property<Boolean>

    @get:Input
    abstract val fir: Property<Boolean>

    @get:InputFiles
    @get:Optional
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val customRuleJars: org.gradle.api.file.ConfigurableFileCollection

    /**
     * Source root directories (e.g. `src/main/kotlin`) used when custom
     * rules are loaded. The JVM-backed custom-rule daemon discovers
     * Kotlin sources by walking these roots for nested `kotlin`/`java`
     * directories; the leaf files in `source` give it nothing to walk.
     */
    @get:InputFiles
    @get:Optional
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val sourceRoots: org.gradle.api.file.ConfigurableFileCollection

    /**
     * This project's exported model. Its absolute paths vary by checkout, so it is
     * keyed through [normalizedModel] and [modelClasspath] instead of by file.
     */
    @get:Internal
    abstract val modelFile: org.gradle.api.file.ConfigurableFileCollection

    @get:Internal
    abstract val cacheDir: DirectoryProperty

    /** `krit { fir { useBuiltDependencies } }`: pass [projectOutputs] to krit. */
    @get:Input
    @get:Optional
    abstract val useBuiltDependencies: Property<Boolean>

    /**
     * Compiled classes (or Android classes jars) of the project dependencies, keyed by
     * project path; krit resolves those projects from them instead of their sources.
     */
    @get:Internal
    abstract val projectOutputs: org.gradle.api.provider.MapProperty<String, List<String>>

    /** The same outputs as an input, ABI-normalized: implementation-only changes do not rerun. */
    @get:CompileClasspath
    @get:Optional
    abstract val dependencyClasses: org.gradle.api.file.ConfigurableFileCollection

    /**
     * `--base-path`: report paths and baseline IDs are relative to it (the project
     * directory by convention). Without it krit uses the first scan path, which for
     * a file list is an arbitrary file, so `kritBaseline` and `kritCheck` IDs differ.
     */
    @get:Internal
    abstract val basePath: DirectoryProperty

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

    /**
     * The krit-fir jar krit runs: `KRIT_FIR_JAR` when set, else the `kritFir` configuration.
     * Empty when FIR is off, or with an explicit binary and no `kritFir` dependency (krit then
     * locates the jar matching its own build, keyed by the binary).
     */
    @get:Classpath
    @get:Optional
    abstract val firJar: org.gradle.api.file.ConfigurableFileCollection

    /** The JDK FIR runs on, passed to krit as `JAVA_HOME`; absent when FIR is off. */
    @get:Nested
    @get:Optional
    abstract val javaLauncher: Property<org.gradle.jvm.toolchain.JavaLauncher>

    /** The model JSON with location-independent paths (see KritModelInputs). */
    @get:Input
    abstract val normalizedModel: Property<String>

    /** The jars the model lists, by content: a re-published jar at the same path is a change. */
    @get:Classpath
    abstract val modelClasspath: org.gradle.api.file.ConfigurableFileCollection

    init {
        // modelFile is @Internal; keep the dependency on the task that writes it. Both
        // keys are derived from the model files' elements, so they are read when the
        // task runs, after the export, and never at configuration time.
        dependsOn(modelFile)
        val models = modelFile.elements.map { files -> files.map { it.asFile } }
        normalizedModel.set(models.map(KritModelInputs::normalized))
        modelClasspath.from(models.map(KritModelInputs::classpath))
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
            // Fall back to SARIF if nothing is enabled (preserves old behavior). The
            // location is the declared sarifOutput, wired at configuration time, so the
            // action never reads the project (configuration cache, Isolated Projects).
            val fallback = sarifReport.orNull?.asFile ?: sarifOutput.orNull?.asFile
                ?: throw GradleException("kritCheck: no report enabled and no sarifOutput location set")
            enabledReports.add("sarif" to fallback)
        }

        // Ensure parent directories exist for all report files
        enabledReports.forEach { (_, file) -> file.parentFile.mkdirs() }

        // One krit run writes every report (--report FORMAT=PATH, #1578). Custom rules
        // need the krit daemon, which renders one report per request, so with custom
        // rule jars each report keeps its own run.
        val batches = if (customRuleJars.isEmpty) listOf(enabledReports) else enabledReports.map { listOf(it) }
        var exitValue = 0
        batches.forEach { reports ->
            val batchExit = runReports(reports, logger) { batch ->
                val stderr = ByteArrayOutputStream()
                val result = execOps.exec {
                    executable = resolveKritExecutable(kritBinary, binaryResolver, toolVersion)
                    args(scanArgs(batch))
                    kritFirEnvironment(firJar, javaLauncher)
                    errorOutput = stderr
                    isIgnoreExitValue = true
                }
                result.exitValue to stderr.toString()
            }
            if (exitValue == 0) exitValue = batchExit
        }

        if (exitValue != 0 && !ignoreFailures.get()) {
            val reportPaths = enabledReports.joinToString(", ") { it.second.absolutePath }
            throw GradleException(
                "krit analysis found issues (exit code $exitValue). " +
                    "See reports at: $reportPaths"
            )
        }
    }

    private fun scanArgs(reports: List<Pair<String, File>>): List<String> = buildList {
        appendReportArgs(reports)
        add("-j"); add(parallel.get().toString())
        if (allRules.get()) add("--all-rules")
        if (config.isPresent) { add("--config"); add(config.get().asFile.absolutePath) }
        if (baseline.isPresent) { add("--baseline"); add(baseline.get().asFile.absolutePath) }
        if (noCache.get()) add("--no-cache")
        if (!typeInference.get()) add("--no-type-inference")
        if (!fir.get()) add("--no-fir")
        if (cacheDir.isPresent) { add("--cache-dir"); add(cacheDir.get().asFile.absolutePath) }
        if (basePath.isPresent) { add("--base-path"); add(basePath.get().asFile.absolutePath) }
        if (useBuiltDependencies.getOrElse(false)) {
            add("--gradle-project-outputs"); add(writeProjectOutputs().absolutePath)
        }
        addCustomRuleJarArgs()
        add("-q")
        appendScanPaths()
    }

    private fun writeProjectOutputs(): File {
        val file = File(temporaryDir, "project-outputs.json")
        file.writeText(groovy.json.JsonOutput.toJson(mapOf(
            "schema" to PROJECT_OUTPUTS_SCHEMA,
            "projects" to projectOutputs.getOrElse(emptyMap()).toSortedMap(),
        )) + "\n")
        return file
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

    private fun MutableList<String>.addCustomRuleJarArgs() {
        appendCustomRuleJarArgs(customRuleJars.files.map { it.absolutePath })
    }

    /**
     * Appends scan paths to the CLI argv: source root directories when
     * custom rules are loaded (the daemon needs roots), otherwise the
     * per-file list (keeps Gradle's incremental input tracking sharp).
     */
    private fun MutableList<String>.appendScanPaths() {
        val needsRoots = !customRuleJars.isEmpty && !sourceRoots.isEmpty
        if (needsRoots) {
            // Skip missing roots so the default `src/main/kotlin` +
            // `src/test/kotlin` convention doesn't fail projects that only
            // ship one of them.
            sourceRoots.files
                .filter { it.exists() }
                .forEach { add(it.absolutePath) }
        } else {
            source.files.forEach { add(it.absolutePath) }
        }
    }

    companion object {
        /** Schema of the `--gradle-project-outputs` file (gradlemodel.ProjectOutputsSchema). */
        internal const val PROJECT_OUTPUTS_SCHEMA = 1

        /**
         * Runs one krit scan that writes every report in [reports] and returns its exit
         * code; [scan] runs krit for a list of reports and returns the exit code and
         * stderr. A krit release without `--report FORMAT=PATH` (0.5.0 and earlier)
         * rejects several reports with "unknown format" (exit 2), so the reports then get
         * one scan each. Any other exit 2 is a configuration or preflight error.
         */
        internal fun runReports(
            reports: List<Pair<String, File>>,
            logger: org.gradle.api.logging.Logger,
            scan: (List<Pair<String, File>>) -> Pair<Int, String>,
        ): Int {
            val (exit, stderr) = scan(reports)
            if (exit == 2 && reports.size > 1 && stderr.contains("unknown format:")) {
                logger.info("krit does not support --report FORMAT=PATH; running one scan per report")
                var first = 0
                reports.forEach { report ->
                    val (single, singleErr) = scan(listOf(report))
                    if (single == 2) throw GradleException("krit configuration/preflight error: ${singleErr.trim()}")
                    if (first == 0) first = single
                }
                return first
            }
            if (exit == 2) throw GradleException("krit configuration/preflight error: ${stderr.trim()}")
            return exit
        }

        /**
         * One report is `--format F -o PATH`, which every krit release understands. Several
         * are one `--report F=PATH` each, written from a single analysis run (krit releases after 0.5.0).
         */
        internal fun MutableList<String>.appendReportArgs(reports: List<Pair<String, File>>) {
            val single = reports.singleOrNull()
            if (single != null) {
                add("--format=${single.first}")
                add("-o"); add(single.second.absolutePath)
            } else {
                reports.forEach { (format, file) -> add("--report"); add("$format=${file.absolutePath}") }
            }
        }

        // Forces --daemon on when jars are present: the CLI hard-errors otherwise
        // (see internal/pipeline/custom_kotlin_rules.go).
        internal fun MutableList<String>.appendCustomRuleJarArgs(jarPaths: Collection<String>) {
            if (jarPaths.isEmpty()) return
            add("--custom-rule-jars")
            add(jarPaths.joinToString(","))
            if ("--daemon" !in this) add("--daemon")
        }
    }
}
