package dev.jasonpearson.krit.gradle

import org.gradle.api.provider.Property
import org.gradle.jvm.toolchain.JavaLauncher

/**
 * How krit runs FIR, configured with `krit { fir { ... } }` (FIR itself is turned on
 * and off with `krit { fir = true }`):
 *
 * ```
 * krit {
 *     fir {
 *         javaLauncher = javaToolchains.launcherFor { languageVersion = JavaLanguageVersion.of(25) }
 *     }
 * }
 * dependencies { kritFir("dev.jasonpearson.krit:krit-fir:<version>") }  // optional pin
 * ```
 *
 * Outside a `krit { }` block, `krit.fir` is the on/off property; reach these options
 * through `krit.firOptions` (for example `krit.firOptions.javaLauncher`).
 */
interface KritFirOptions {
    /**
     * The JDK FIR runs on; its home is passed to krit as `JAVA_HOME`. Default: the
     * Java toolchain for the newer of 21 (FIR's minimum) and the JVM Gradle runs on,
     * so a Gradle on JDK 17 gets a JDK 21 toolchain rather than failing.
     */
    val javaLauncher: Property<JavaLauncher>

    /**
     * Resolve project dependencies from their compiled classes (built through Gradle,
     * so from the build cache when available) instead of letting krit compile their
     * sources with every dependent project's check. Their classes are an ABI-normalized
     * input: an upstream change that keeps the ABI does not rerun the check. Needs a
     * krit that supports `--gradle-project-outputs` (after 0.5.0). Default: false.
     */
    val useBuiltDependencies: Property<Boolean>

    /**
     * How many warm FIR JVMs the build's krit tasks share (see [KritFirDaemonPool]).
     * Also the most krit tasks with FIR that run at once; capped by `--max-workers`.
     * Read from the first project that applies the plugin. Default: 2.
     */
    val maxDaemons: Property<Int>

    /** Heap cap of each FIR JVM, such as "4g" (krit's `KRIT_FIR_XMX`). Default: a quarter of memory. */
    val maxHeap: Property<String>
}

abstract class DefaultKritFirOptions : KritFirOptions

/** Maven coordinates of the krit-fir jar released with each krit version. */
internal const val KRIT_FIR_MODULE = "dev.jasonpearson.krit:krit-fir"

/**
 * Points krit at the task's krit-fir jar and FIR JDK: krit reads `KRIT_FIR_JAR`, and
 * `JAVA_HOME` before `PATH` (internal/jdk). Unset values keep the inherited environment.
 */
internal fun org.gradle.process.ExecSpec.kritFirEnvironment(
    firJar: org.gradle.api.file.FileCollection,
    javaLauncher: Property<JavaLauncher>,
    poolSlot: String? = null,
    maxHeap: Property<String>? = null,
) {
    val jars = firJar.files
    if (jars.size > 1) {
        throw org.gradle.api.GradleException(
            "krit: the kritFir configuration resolved to ${jars.size} files; declare exactly one krit-fir jar: " +
                jars.joinToString { it.name }
        )
    }
    jars.singleOrNull()?.let { environment("KRIT_FIR_JAR", it.absolutePath) }
    javaLauncher.orNull?.let { environment("JAVA_HOME", it.metadata.installationPath.asFile.absolutePath) }
    poolSlot?.let { environment(KritFirDaemonPool.POOL_ENV, it) }
    maxHeap?.orNull?.let { heap ->
        if (!Regex("[0-9]+[kKmMgG]").matches(heap)) {
            throw org.gradle.api.GradleException("krit fir.maxHeap must look like 4g or 2048m, not '$heap'")
        }
        environment(KritFirDaemonPool.HEAP_ENV, heap)
    }
}

/** Runs [block] with a leased FIR daemon pool slot when [pool] is set (FIR on), else with none. */
internal fun <T> withFirDaemonSlot(pool: Property<KritFirDaemonPool>, block: (String?) -> T): T =
    pool.orNull?.withSlot { slot -> block(slot) } ?: block(null)
