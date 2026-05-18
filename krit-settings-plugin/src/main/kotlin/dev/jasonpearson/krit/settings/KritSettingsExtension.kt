package dev.jasonpearson.krit.settings

import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.Property
import org.gradle.api.provider.SetProperty

/**
 * Root-level DSL for the `dev.jasonpearson.krit.settings` plugin.
 *
 * Configured from `settings.gradle.kts`:
 * ```
 * plugins {
 *     id("dev.jasonpearson.krit.settings") version "0.2.0"
 * }
 *
 * krit {
 *     config = file("krit.yml")        // shared config across all subprojects
 *     baseline = file("krit-baseline.xml")
 *     ignoreFailures = false
 *     skip(":rule-bundle", ":generated-stubs")
 * }
 * ```
 *
 * The settings plugin then auto-applies `dev.jasonpearson.krit` to every
 * subproject that brings in a JVM language plugin (Kotlin JVM, Kotlin Android,
 * Android application/library) and seeds each subproject's `krit { }` block
 * with the values configured here. Per-subproject overrides still work — the
 * inherited values are wired as conventions, not hard sets.
 */
interface KritSettingsExtension {
    /**
     * Path to the krit.yml config shared across subprojects. Optional — when
     * unset the per-project CLI auto-discovers `krit.yml` / `.krit.yml`.
     */
    val config: RegularFileProperty

    /** Default value for each subproject's `krit.ignoreFailures`. Default: false. */
    val ignoreFailures: Property<Boolean>

    /** Default baseline path inherited by each subproject. */
    val baseline: RegularFileProperty

    /**
     * Project paths (e.g., `":rule-bundle"`) to skip when auto-applying the
     * project plugin. Useful for rule-producer modules or generated-code
     * modules where running krit on the sources is not desired.
     */
    val skipped: SetProperty<String>

    /** Convenience for adding skip entries. */
    fun skip(vararg paths: String)
}

abstract class DefaultKritSettingsExtension : KritSettingsExtension {
    override fun skip(vararg paths: String) {
        skipped.addAll(*paths)
    }
}
