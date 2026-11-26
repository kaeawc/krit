package dev.jasonpearson.krit.gradle

import dev.jasonpearson.krit.gradle.KritCheckTask.Companion.appendCustomRuleJarArgs
import dev.jasonpearson.krit.gradle.KritCheckTask.Companion.appendReportArgs
import dev.jasonpearson.krit.gradle.KritCheckTask.Companion.runReports
import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
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
import org.gradle.api.tasks.TaskAction
import org.gradle.language.base.plugins.LifecycleBasePlugin
import org.gradle.process.ExecOperations
import java.io.ByteArrayOutputStream
import java.io.File
import javax.inject.Inject

/**
 * Whole-build krit scan: one krit process (and so one FIR session, or one set of
 * module-shard workers) over the build root, reading every project's exported
 * Gradle model. Registered as `kritCheckAll` by the `dev.jasonpearson.krit.aggregate`
 * plugin, which the settings plugin applies to the root project.
 *
 * The models reach this task through variant-aware project dependencies (see
 * [KritAggregatePlugin]), so the task never touches another project's state and
 * stays Isolated Projects safe.
 */
@CacheableTask
abstract class KritCheckAllTask @Inject constructor(
    private val execOps: ExecOperations,
) : DefaultTask() {

    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val kritBinary: RegularFileProperty

    @get:Input
    abstract val toolVersion: Property<String>

    @get:Internal
    abstract val binaryResolver: Property<KritBinaryResolver>

    /**
     * Every project's `kritExportModel` output; they all live in `<root>/.krit/gradle-model`.
     * Keyed through [normalizedModel] and [modelClasspath], not by file (see KritModelInputs).
     */
    @get:Internal
    abstract val modelFiles: ConfigurableFileCollection

    /**
     * The files krit reads under [scanRoot], so the task is up to date only when
     * none of them changed. Build directories and krit's own state are excluded.
     */
    @get:InputFiles
    @get:SkipWhenEmpty
    @get:IgnoreEmptyDirectories
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val sources: ConfigurableFileCollection

    /** Directory krit scans: the build root. Its content is tracked through [sources]. */
    @get:Internal
    abstract val scanRoot: DirectoryProperty

    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val config: RegularFileProperty

    @get:InputFile
    @get:Optional
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val baseline: RegularFileProperty

    @get:Input abstract val allRules: Property<Boolean>
    @get:Input abstract val ignoreFailures: Property<Boolean>
    // Neither changes the findings; parallel defaults to the CPU count (see KritCheckTask).
    @get:Internal abstract val parallel: Property<Int>
    @get:Internal abstract val noCache: Property<Boolean>
    @get:Input abstract val typeInference: Property<Boolean>
    @get:Input abstract val fir: Property<Boolean>

    /**
     * `KRIT_MODULE_SHARDS` for the scan (`auto`, a worker count, `merged`, `isolate`);
     * unset keeps krit's default whole-repository compile. See docs/module-sharding.md.
     */
    @get:Input
    @get:Optional
    abstract val moduleShards: Property<String>

    @get:InputFiles
    @get:Optional
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val customRuleJars: ConfigurableFileCollection

    @get:Input abstract val sarifRequired: Property<Boolean>
    @get:OutputFile @get:Optional abstract val sarifOutput: RegularFileProperty
    @get:Input abstract val jsonRequired: Property<Boolean>
    @get:OutputFile @get:Optional abstract val jsonOutput: RegularFileProperty
    @get:Input abstract val plainRequired: Property<Boolean>
    @get:OutputFile @get:Optional abstract val plainOutput: RegularFileProperty
    @get:Input abstract val checkstyleRequired: Property<Boolean>
    @get:OutputFile @get:Optional abstract val checkstyleOutput: RegularFileProperty

    /**
     * The krit-fir jar krit runs: `KRIT_FIR_JAR` when set, else the `kritFir` configuration.
     * Empty when FIR is off, or with an explicit binary and no `kritFir` dependency (krit then
     * locates the jar matching its own build, keyed by the binary).
     */
    @get:Classpath
    @get:Optional
    abstract val firJar: ConfigurableFileCollection

    /** The JDK FIR runs on, passed to krit as `JAVA_HOME`; absent when FIR is off. */
    @get:Nested
    @get:Optional
    abstract val javaLauncher: Property<org.gradle.jvm.toolchain.JavaLauncher>

    /** Shared warm FIR JVMs; absent when FIR is off. */
    @get:Internal
    abstract val firDaemonPool: Property<KritFirDaemonPool>

    /** `fir { maxHeap }`: does not change the findings, so not an input. */
    @get:Internal
    abstract val firMaxHeap: Property<String>

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
        dependsOn(modelFiles)
        val models = modelFiles.elements.map { files -> files.map { it.asFile } }
        normalizedModel.set(models.map(KritModelInputs::normalized))
        modelClasspath.from(models.map(KritModelInputs::classpath))
        group = LifecycleBasePlugin.VERIFICATION_GROUP
        description = "Run one krit analysis over the whole build with every project's Gradle model"
    }

    @TaskAction
    fun check() {
        val reports = listOfNotNull(
            report("sarif", sarifRequired, sarifOutput),
            report("json", jsonRequired, jsonOutput),
            report("plain", plainRequired, plainOutput),
            report("checkstyle", checkstyleRequired, checkstyleOutput),
        ).ifEmpty {
            listOfNotNull(sarifOutput.orNull?.let { "sarif" to it.asFile })
        }
        if (reports.isEmpty()) throw GradleException("kritCheckAll: no report output configured")
        reports.forEach { (_, file) -> file.parentFile.mkdirs() }

        val executable = resolveKritExecutable(kritBinary, binaryResolver, toolVersion)
        val root = scanRoot.get().asFile
        val exitValue = runReports(reports, logger) { batch ->
            val stderr = ByteArrayOutputStream()
            val result = withFirDaemonSlot(firDaemonPool) { slot ->
                execOps.exec {
                    this.executable = executable
                    workingDir = root
                    moduleShards.orNull?.takeIf { it.isNotBlank() }?.let { environment("KRIT_MODULE_SHARDS", it) }
                    args(scanArgs(batch, root))
                    kritFirEnvironment(firJar, javaLauncher, slot, firMaxHeap)
                    errorOutput = stderr
                    isIgnoreExitValue = true
                }
            }
            result.exitValue to stderr.toString()
        }
        projectSummary(reports)?.let { logger.lifecycle(it) }
        if (exitValue != 0 && !ignoreFailures.get()) {
            throw GradleException(
                "krit analysis found issues (exit code $exitValue). " +
                    "See reports at: ${reports.joinToString(", ") { it.second.absolutePath }}"
            )
        }
    }

    /**
     * The findings per Gradle project of the whole-build report (#1651), read from the
     * report's `byProject` counts: JSON `summary.byProject`, else SARIF
     * `runs[0].properties.byProject` (krit after 0.5.0 writes them when it loads the model).
     * Null when no report has them, or nothing was found.
     */
    private fun projectSummary(reports: List<Pair<String, File>>): String? {
        val byProject = reports.firstNotNullOfOrNull { (format, file) ->
            if (!file.isFile) return@firstNotNullOfOrNull null
            val parsed = runCatching { groovy.json.JsonSlurper().parse(file) as? Map<*, *> }.getOrNull()
            when (format) {
                "json" -> (parsed?.get("summary") as? Map<*, *>)?.get("byProject")
                "sarif" -> ((parsed?.get("runs") as? List<*>)?.firstOrNull() as? Map<*, *>)
                    ?.let { (it["properties"] as? Map<*, *>)?.get("byProject") }
                else -> null
            } as? Map<*, *>
        } ?: return null
        return formatProjectSummary(byProject)
    }

    private fun report(format: String, required: Property<Boolean>, output: RegularFileProperty): Pair<String, File>? =
        if (required.getOrElse(false)) output.orNull?.let { format to it.asFile } else null

    private fun scanArgs(reports: List<Pair<String, File>>, root: File): List<String> = buildList {
        appendReportArgs(reports)
        add("-j"); add(parallel.get().toString())
        if (allRules.get()) add("--all-rules")
        if (config.isPresent) { add("--config"); add(config.get().asFile.absolutePath) }
        if (baseline.isPresent) { add("--baseline"); add(baseline.get().asFile.absolutePath) }
        if (noCache.get()) add("--no-cache")
        if (!typeInference.get()) add("--no-type-inference")
        if (!fir.get()) add("--no-fir")
        add("--base-path"); add(root.absolutePath)
        appendCustomRuleJarArgs(customRuleJars.files.map { it.absolutePath })
        add("-q")
        add(root.absolutePath)
    }

    internal companion object {
        /**
         * One line per project with findings, most findings first, as
         * `  :app  3 (1 error, 2 warnings)`; findings outside every project are listed last.
         */
        fun formatProjectSummary(byProject: Map<*, *>): String? {
            data class Row(val project: String, val findings: Int, val errors: Int, val warnings: Int)
            fun count(entry: Any?, key: String) = ((entry as? Map<*, *>)?.get(key) as? Number)?.toInt() ?: 0
            val rows = byProject.map { (project, counts) ->
                Row(project.toString(), count(counts, "findings"), count(counts, "errors"), count(counts, "warnings"))
            }.filter { it.findings > 0 }
                .sortedWith(compareBy<Row> { it.project.isEmpty() }.thenByDescending { it.findings }.thenBy { it.project })
            if (rows.isEmpty()) return null
            val label = { row: Row -> row.project.ifEmpty { "(outside every project)" } }
            val width = rows.maxOf { label(it).length }
            fun plural(n: Int, word: String) = "$n $word" + if (n == 1) "" else "s"
            return "krit: findings by project\n" + rows.joinToString("\n") { row ->
                "  ${label(row).padEnd(width)}  ${row.findings} (${plural(row.errors, "error")}, ${plural(row.warnings, "warning")})"
            }
        }

        /** Files under the build root that a whole-build scan reads. */
        val SOURCE_INCLUDES = listOf(
            "**/*.kt", "**/*.kts", "**/*.java", "**/*.xml", "**/*.toml", "**/*.gradle",
            "**/krit.yml", "**/.krit.yml",
        )

        /** Build outputs and tool state: never analysis input, and often other tasks' outputs. */
        val SOURCE_EXCLUDES = listOf(
            "**/build/**", "**/.gradle/**", "**/.krit/**", "**/.git/**", "**/.idea/**",
            "**/node_modules/**", "**/.kotlin/**",
        )
    }
}
