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
}
