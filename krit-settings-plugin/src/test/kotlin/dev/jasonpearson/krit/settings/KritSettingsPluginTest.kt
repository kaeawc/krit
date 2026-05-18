package dev.jasonpearson.krit.settings

import dev.jasonpearson.krit.gradle.KritPlugin
import org.gradle.testfixtures.ProjectBuilder
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test

class KritSettingsPluginTest {

    @Test
    fun `auto-apply list matches the project plugin's authoritative LANGUAGE_PLUGIN_IDS`() {
        // Drift guard: the settings plugin only auto-applies KritPlugin to
        // subprojects with plugin IDs that KritPlugin itself reacts to (i.e.
        // those that trigger per-source-set / per-variant task wiring). If
        // KritPlugin learns to handle a new language plugin, the settings
        // plugin should pick it up automatically — and vice versa.
        assertEquals(KritPlugin.LANGUAGE_PLUGIN_IDS, KritSettingsPlugin.JVM_LANGUAGE_PLUGIN_IDS)
    }

    @Test
    fun `settings extension is a managed type with sensible defaults`() {
        val project = ProjectBuilder.builder().build()
        val ext = project.objects.newInstance(DefaultKritSettingsExtension::class.java)
        // Conventions are wired in KritSettingsPlugin.apply; reproduce them
        // here so the extension can be exercised without a Settings host.
        ext.ignoreFailures.convention(false)
        ext.skipped.convention(emptySet())

        assertNotNull(ext.config, "config property should be present")
        assertNotNull(ext.baseline, "baseline property should be present")
        assertEquals(false, ext.ignoreFailures.get())
        assertEquals(emptySet<String>(), ext.skipped.get())
        assertFalse(ext.config.isPresent, "config should default unset")
        assertFalse(ext.baseline.isPresent, "baseline should default unset")
    }

    @Test
    fun `skip(varargs) adds entries to the skipped set`() {
        val project = ProjectBuilder.builder().build()
        val ext = project.objects.newInstance(DefaultKritSettingsExtension::class.java)
        ext.skipped.convention(emptySet())

        ext.skip(":rule-bundle", ":generated-stubs")

        assertTrue(":rule-bundle" in ext.skipped.get(),
            "skipped must contain :rule-bundle; was ${ext.skipped.get()}")
        assertTrue(":generated-stubs" in ext.skipped.get())
        assertEquals(2, ext.skipped.get().size)
    }

    @Test
    fun `skip is additive across calls`() {
        val project = ProjectBuilder.builder().build()
        val ext = project.objects.newInstance(DefaultKritSettingsExtension::class.java)
        ext.skipped.convention(emptySet())

        ext.skip(":first")
        ext.skip(":second", ":third")

        assertEquals(setOf(":first", ":second", ":third"), ext.skipped.get())
    }
}
