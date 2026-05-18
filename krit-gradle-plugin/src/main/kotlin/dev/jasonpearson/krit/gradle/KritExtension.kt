package dev.jasonpearson.krit.gradle

import org.gradle.api.Action
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.model.ObjectFactory
import org.gradle.api.provider.Property
import javax.inject.Inject

/**
 * DSL extension for configuring the krit Kotlin static analysis plugin.
 *
 * The common-path surface is intentionally small — apply the plugin, drop a
 * `krit.yml` next to the build file, and that's enough for most projects.
 * The few settings most users want:
 *
 * ```
 * krit {
 *     config = file("krit.yml")          // optional; auto-discovered when omitted
 *     baseline = file("krit-baseline.xml")
 *     ignoreFailures = false             // gate the build on findings
 *     reports {
 *         sarif.required = true
 *         json.required = true
 *     }
 * }
 * ```
 *
 * Escape hatches (binary path, parallelism, cache toggles, fix level, etc.)
 * live under [advanced]. Project-level custom-rule wiring goes through the
 * `kritCustomRules` configuration in the `dependencies` block:
 *
 * ```
 * dependencies { kritCustomRules(project(":custom-rules")) }
 * ```
 *
 * For one-off jars or task outputs that don't fit the dependency-block model,
 * append directly to [customRuleJars] (a `ConfigurableFileCollection`):
 *
 * ```
 * krit { customRuleJars.from(file("libs/my-rules.jar")) }
 * ```
 */
abstract class KritExtension @Inject constructor(objects: ObjectFactory) {

    /** Path to krit.yml. Optional — when unset, the CLI auto-discovers from the project root. */
    abstract val config: RegularFileProperty

    /** Fail the build when findings exceed the configured severity threshold. Default: false. */
    abstract val ignoreFailures: Property<Boolean>

    /** Baseline file for suppressing known issues. */
    abstract val baseline: RegularFileProperty

    /** Kotlin custom-rule jars to load through krit-types. Populated by `kritCustomRules` deps. */
    abstract val customRuleJars: ConfigurableFileCollection

    /** Report format configuration. */
    val reports: KritReports = objects.newInstance(DefaultKritReports::class.java)

    /** Configure reports via DSL block. */
    fun reports(action: Action<KritReports>) {
        action.execute(reports)
    }

    /** Advanced/escape-hatch settings — see [KritAdvanced]. */
    val advanced: KritAdvanced = objects.newInstance(DefaultKritAdvanced::class.java)

    /** Configure advanced settings via DSL block. */
    fun advanced(action: Action<KritAdvanced>) {
        action.execute(advanced)
    }
}
