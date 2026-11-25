package dev.jasonpearson.krit.gradle

import org.gradle.api.tasks.Internal
import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.plugins.BasePlugin
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Classpath
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.Nested
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Optional
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import org.gradle.work.DisableCachingByDefault
import javax.inject.Inject
import dev.jasonpearson.krit.gradle.KritCheckTask.Companion.appendCustomRuleJarArgs

/**
 * Gradle task that invokes the krit binary to create a baseline file.
 *
 * A baseline captures all current findings so that subsequent runs only report
 * new issues. This is useful when adopting krit on an existing codebase.
 */
@DisableCachingByDefault(because = "Captures the current findings on every run")
abstract class KritBaselineTask @Inject constructor(
    private val execOps: ExecOperations,
) : DefaultTask() {

    @get:InputFiles
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val source: ConfigurableFileCollection

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

    /** Same FIR setting as `kritCheck`, so the baseline holds the findings it reports. */
    @get:Input
    abstract val fir: Property<Boolean>

    @get:InputFiles
    @get:Optional
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val customRuleJars: ConfigurableFileCollection

    /** `--base-path` for baseline IDs; must match `kritCheck`'s (the project directory). */
    @get:Internal
    abstract val basePath: org.gradle.api.file.DirectoryProperty

    /** The krit-fir jar and FIR JDK, as for `kritCheck`. */
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

    init {
        group = BasePlugin.BUILD_GROUP
        description = "Create a krit baseline file from current findings"
    }

    @TaskAction
    fun createBaseline() {
        // The default source roots (src/main/kotlin, src/test/kotlin) are directories
        // that need not exist; krit rejects a missing scan path.
        val sourceFiles = source.files.filter { it.exists() }
        if (sourceFiles.isEmpty()) {
            logger.info("No source files found, skipping baseline creation")
            return
        }

        val outputFile = baselineFile.get().asFile
        outputFile.parentFile.mkdirs()

        val args = buildList {
            // --create-baseline takes the output path; a separate --baseline flag
            // would be read as its value (#1639).
            add("--create-baseline"); add(outputFile.absolutePath)
            if (basePath.isPresent) { add("--base-path"); add(basePath.get().asFile.absolutePath) }
            add("-j"); add(parallel.get().toString())
            if (allRules.get()) add("--all-rules")
            if (config.isPresent) { add("--config"); add(config.get().asFile.absolutePath) }
            if (noCache.get()) add("--no-cache")
            if (!typeInference.get()) add("--no-type-inference")
            if (!fir.getOrElse(true)) add("--no-fir")
            appendCustomRuleJarArgs(customRuleJars.files.map { it.absolutePath })
            add("-q")
            sourceFiles.forEach { add(it.absolutePath) }
        }

        val result = withFirDaemonSlot(firDaemonPool) { slot ->
            execOps.exec {
                executable = resolveKritExecutable(kritBinary, binaryResolver, toolVersion)
                args(args)
                kritFirEnvironment(firJar, javaLauncher, slot, firMaxHeap)
                isIgnoreExitValue = true
            }
        }

        if (result.exitValue != 0) {
            throw GradleException(
                "krit baseline creation failed (exit code ${result.exitValue})"
            )
        }

        logger.lifecycle("krit baseline created at ${outputFile.absolutePath}")
    }
}
