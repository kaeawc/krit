package dev.jasonpearson.krit.gradle

import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.Property

/**
 * Escape-hatch settings for projects that need to override krit's automatic
 * configuration. The common path (Kotlin/Android plugin applied, default
 * `build/reports/krit` output, bundled binary) does not need any of these.
 *
 * Access via:
 * ```
 * krit {
 *     advanced {
 *         parallel = 4
 *         binary = file("/path/to/krit")
 *     }
 * }
 * ```
 */
interface KritAdvanced {
    /** Krit binary version to download. Defaults to the plugin's bundled version. */
    val toolVersion: Property<String>

    /** Local krit binary path. Overrides the downloaded binary when set. */
    val binary: RegularFileProperty

    /** Number of parallel jobs (maps to `-j`). Defaults to CPU count. */
    val parallel: Property<Int>

    /** Disable the incremental analysis cache (maps to `--no-cache`). Default: false. */
    val noCache: Property<Boolean>

    /** Enable source-level type inference. Default: true. */
    val typeInference: Property<Boolean>

    /** Auto-fix safety level: `cosmetic`, `idiomatic` (default), or `semantic`. */
    val fixLevel: Property<String>

    /** Enable all rules including opt-in (maps to `--all-rules`). Default: false. */
    val allRules: Property<Boolean>

    /**
     * Source directories scanned by the aggregate `kritCheck` task. Auto-derived
     * from the Kotlin JVM / Android source sets; only override for non-standard
     * layouts. Default: `src/main/kotlin`, `src/test/kotlin`.
     */
    val source: ConfigurableFileCollection

    /** Reports output directory. Default: `build/reports/krit`. */
    val reportsDir: DirectoryProperty
}

abstract class DefaultKritAdvanced : KritAdvanced
