package dev.krit.gradle

import org.gradle.api.Action
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.model.ObjectFactory
import org.gradle.api.provider.Property
import javax.inject.Inject

/**
 * DSL extension for configuring the krit Kotlin static analysis plugin.
 *
 * Usage in build.gradle.kts:
 * ```
 * krit {
 *     toolVersion.set("0.2.0")
 *     config.set(file("krit.yml"))
 *     ignoreFailures.set(false)
 *     reports {
 *         sarif { required.set(true) }
 *         json { required.set(false) }
 *     }
 * }
 * ```
 */
abstract class KritExtension @Inject constructor(objects: ObjectFactory) {

    /** Krit binary version to download. Default: plugin's bundled version constant. */
    abstract val toolVersion: Property<String>

    /** Path to krit.yml config file. */
    abstract val config: RegularFileProperty

    /** Severity threshold -- fail the build if findings at or above this level exist. */
    abstract val ignoreFailures: Property<Boolean>

    /** Enable all rules including opt-in (maps to --all-rules). */
    abstract val allRules: Property<Boolean>

    /** Baseline file for suppressing known issues. */
    abstract val baseline: RegularFileProperty

    /** Source directories to analyze. */
    abstract val source: ConfigurableFileCollection

    /** Reports output directory. */
    abstract val reportsDir: DirectoryProperty

    /** Auto-fix level: cosmetic, idiomatic, or semantic. */
    abstract val fixLevel: Property<String>

    /** Path to a local krit binary (skips download). */
    abstract val binary: RegularFileProperty

    /** Number of parallel jobs (maps to -j). Default: CPU count. */
    abstract val parallel: Property<Int>

    /** Disable incremental analysis cache (maps to --no-cache). */
    abstract val noCache: Property<Boolean>

    /** Enable type inference (default true). */
    abstract val typeInference: Property<Boolean>

    /** Report format configuration. */
    val reports: KritReports = objects.newInstance(DefaultKritReports::class.java)

    /** Configure reports via DSL block. */
    fun reports(action: Action<KritReports>) {
        action.execute(reports)
    }
}
