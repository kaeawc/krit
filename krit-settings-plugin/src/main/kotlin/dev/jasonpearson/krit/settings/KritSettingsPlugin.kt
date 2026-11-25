package dev.jasonpearson.krit.settings

import dev.jasonpearson.krit.gradle.KritAggregatePlugin
import dev.jasonpearson.krit.gradle.KritExtension
import dev.jasonpearson.krit.gradle.KritPlugin
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.initialization.ProjectDescriptor
import org.gradle.api.initialization.Settings
import org.gradle.api.provider.Provider
import org.gradle.api.file.RegularFile

/**
 * Settings-level plugin that auto-applies [KritPlugin] to every Kotlin / Java /
 * Android subproject under a single root configuration. See
 * [KritSettingsExtension] for the configuration surface.
 *
 * The hook runs through `gradle.lifecycle.beforeProject`, the Project-Isolation
 * safe replacement for `gradle.beforeProject` / `subprojects { }`. Each
 * matching subproject receives [KritPlugin] exactly once, and the inherited
 * settings flow in as conventions so per-subproject overrides remain
 * authoritative.
 *
 * The root project also gets [KritAggregatePlugin]'s `kritCheckAll`, with a
 * `kritAggregate` dependency on every included, non-skipped project: one krit
 * scan of the whole build with every project's exported model.
 */
class KritSettingsPlugin : Plugin<Settings> {

    override fun apply(settings: Settings) {
        val ext = settings.extensions.create(
            "krit",
            DefaultKritSettingsExtension::class.java,
        )
        ext.ignoreFailures.convention(false)
        ext.skipped.convention(emptySet())

        // Capture providers (not the resolved values) outside the per-project
        // lambda. Providers are lazy: `.get()` inside `beforeProject` reads
        // the final value after settings.gradle.kts has finished configuring
        // the extension. Capturing values at apply()-time would freeze the
        // empty defaults before the user's `krit { ... }` block runs. The
        // lambda intentionally does NOT capture `Settings`-scoped state —
        // Project-Isolation safe.
        val skippedProvider: Provider<Set<String>> = ext.skipped
        val configProvider: Provider<RegularFile> = ext.config
        val baselineProvider: Provider<RegularFile> = ext.baseline
        val ignoreFailuresProvider: Provider<Boolean> = ext.ignoreFailures

        // The root project's kritCheckAll aggregates every included project's model.
        // The project list is settings data, complete once the settings script has
        // run; it is captured as a value because isolated actions cannot hold descriptors.
        val projectPaths = ext.projectPaths
        settings.gradle.settingsEvaluated { projectPaths.set(allProjectPaths(rootProject)) }

        // `lifecycle.beforeProject` runs with the Project as the implicit
        // receiver; bind it to a local so `plugins.withId { ... }` (which
        // shadows the receiver with `AppliedPlugin`) can still reference it.
        settings.gradle.lifecycle.beforeProject {
            val project = this
            if (project.path in skippedProvider.get()) return@beforeProject
            if (project.path == ":") {
                registerAggregate(project, projectPaths.get() - skippedProvider.get(),
                    configProvider, baselineProvider, ignoreFailuresProvider)
            }
            JVM_LANGUAGE_PLUGIN_IDS.forEach { id ->
                project.plugins.withId(id) {
                    autoApplyKrit(project, configProvider, baselineProvider, ignoreFailuresProvider)
                }
            }
        }
    }

    private fun registerAggregate(
        root: Project,
        paths: Collection<String>,
        config: Provider<RegularFile>,
        baseline: Provider<RegularFile>,
        ignoreFailures: Provider<Boolean>,
    ) {
        root.plugins.apply(KritAggregatePlugin::class.java)
        inheritConventions(root, config, baseline, ignoreFailures)
        paths.filter { it != ":" }.sorted().forEach { path ->
            root.dependencies.add(KritAggregatePlugin.AGGREGATE_CONFIGURATION,
                root.dependencies.project(mapOf("path" to path)))
        }
    }

    private fun allProjectPaths(descriptor: ProjectDescriptor): List<String> =
        listOf(descriptor.path) + descriptor.children.flatMap(::allProjectPaths)

    private fun autoApplyKrit(
        project: Project,
        config: Provider<RegularFile>,
        baseline: Provider<RegularFile>,
        ignoreFailures: Provider<Boolean>,
    ) {
        if (project.plugins.hasPlugin(KritPlugin::class.java)) return
        project.plugins.apply(KritPlugin::class.java)
        inheritConventions(project, config, baseline, ignoreFailures)
    }

    private fun inheritConventions(
        project: Project,
        config: Provider<RegularFile>,
        baseline: Provider<RegularFile>,
        ignoreFailures: Provider<Boolean>,
    ) {
        val kritExt = project.extensions.getByType(KritExtension::class.java)
        if (config.isPresent) kritExt.config.convention(config)
        if (baseline.isPresent) kritExt.baseline.convention(baseline)
        kritExt.ignoreFailures.convention(ignoreFailures)
    }

    internal companion object {
        /**
         * Plugin IDs that trigger auto-application. Mirrors
         * [KritPlugin.LANGUAGE_PLUGIN_IDS] verbatim — the unit test pins this
         * equality so the two cannot drift.
         */
        internal val JVM_LANGUAGE_PLUGIN_IDS: List<String> = KritPlugin.LANGUAGE_PLUGIN_IDS
    }
}
