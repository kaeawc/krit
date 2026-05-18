package dev.jasonpearson.krit.settings

import dev.jasonpearson.krit.gradle.KritExtension
import dev.jasonpearson.krit.gradle.KritPlugin
import org.gradle.api.Plugin
import org.gradle.api.Project
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

        // `lifecycle.beforeProject` runs with the Project as the implicit
        // receiver; bind it to a local so `plugins.withId { ... }` (which
        // shadows the receiver with `AppliedPlugin`) can still reference it.
        settings.gradle.lifecycle.beforeProject {
            val project = this
            if (project.path in skippedProvider.get()) return@beforeProject
            JVM_LANGUAGE_PLUGIN_IDS.forEach { id ->
                project.plugins.withId(id) {
                    autoApplyKrit(project, configProvider, baselineProvider, ignoreFailuresProvider)
                }
            }
        }
    }

    private fun autoApplyKrit(
        project: Project,
        config: Provider<RegularFile>,
        baseline: Provider<RegularFile>,
        ignoreFailures: Provider<Boolean>,
    ) {
        if (project.plugins.hasPlugin(KritPlugin::class.java)) return
        project.plugins.apply(KritPlugin::class.java)
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
