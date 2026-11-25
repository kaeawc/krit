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
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.gradle.process.ExecOperations
import org.gradle.work.DisableCachingByDefault
import javax.inject.Inject

/**
 * Gradle task that invokes the krit binary with --fix to apply auto-fixes.
 *
 * The fix level controls which categories of fixes are applied:
 * - cosmetic: whitespace, formatting, trailing commas
 * - idiomatic: Kotlin idiom improvements (default)
 * - semantic: fixes that may change runtime behavior
 */
@DisableCachingByDefault(because = "Rewrites source files in place")
abstract class KritFormatTask @Inject constructor(
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

    @get:Input
    @get:Optional
    abstract val fixLevel: Property<String>

    @get:Input
    abstract val parallel: Property<Int>

    @get:Input
    abstract val noCache: Property<Boolean>

    @get:Input
    abstract val typeInference: Property<Boolean>

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
        description = "Apply krit auto-fixes to Kotlin sources"
        // Format tasks are never cacheable -- they modify source files in place
        outputs.upToDateWhen { false }
    }

    @TaskAction
    fun format() {
        // The default source roots (src/main/kotlin, src/test/kotlin) are directories
        // that need not exist; krit rejects a missing scan path.
        val sourceFiles = source.files.filter { it.exists() }
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
                "krit format failed (exit code ${result.exitValue})"
            )
        }

        logger.lifecycle("krit format applied fixes to ${sourceFiles.size} source files")
    }
}
