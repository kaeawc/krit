package dev.jasonpearson.krit.gradle

import org.gradle.api.Action
import org.gradle.api.Project
import org.gradle.api.UnknownTaskException
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.model.ObjectFactory
import org.gradle.api.provider.Property
import org.gradle.jvm.tasks.Jar
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
 * `kritCustomRules` configuration in the `dependencies` block.
 */
abstract class KritExtension @Inject constructor(
    private val project: Project,
    objects: ObjectFactory,
) {

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

    /**
     * Add Kotlin custom-rule jars or projects that produce a jar task.
     *
     * For `Project` notations, prefers `kritRuleJar` (the stamped archive
     * registered by `dev.jasonpearson.krit.custom`) over the default `jar`
     * task — only the former carries the `Krit-SDK-Version` / `Krit-Vendor-Id`
     * manifest attributes the daemon reads. The sibling project is configured
     * before lookup so its lazily registered tasks are visible.
     *
     * Prefer the variant-aware form for project deps:
     * ```
     * dependencies { kritCustomRules(project(":custom-rules")) }
     * ```
     * which routes through Gradle's dependency graph instead of cross-project
     * task lookup.
     */
    fun customRules(vararg notations: Any) {
        notations.forEach { notation ->
            if (notation is Project) {
                project.evaluationDependsOn(notation.path)
                val jarTask = try {
                    notation.tasks.named("kritRuleJar", Jar::class.java)
                } catch (_: UnknownTaskException) {
                    notation.tasks.named("jar", Jar::class.java)
                }
                customRuleJars.from(jarTask.flatMap { it.archiveFile })
                customRuleJars.builtBy(jarTask)
            } else {
                customRuleJars.from(notation)
            }
        }
    }
}
