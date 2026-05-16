package dev.jasonpearson.krit.custom

import org.gradle.api.Project
import org.gradle.api.provider.Provider
import org.gradle.api.tasks.bundling.Jar
import org.gradle.testfixtures.ProjectBuilder
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

class KritCustomRulePluginTest {

    @TempDir
    lateinit var projectDir: File

    private fun newProject(): Project {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()
        project.pluginManager.apply(KritCustomRulePlugin::class.java)
        return project
    }

    @Test
    fun `kotlin jvm plugin is auto-applied`() {
        val project = newProject()
        assertTrue(
            project.pluginManager.hasPlugin("org.jetbrains.kotlin.jvm"),
            "Kotlin JVM plugin should be applied automatically"
        )
    }

    @Test
    fun `kritCustomRules extension is registered with sensible defaults`() {
        val project = newProject()
        val extension = project.extensions.findByType(KritCustomRulesExtension::class.java)
        assertNotNull(extension, "kritCustomRules extension should be registered")
        assertEquals("custom", extension!!.vendorId.get())
        assertEquals("warning", extension.defaultSeverity.get())
        assertEquals(BuildConfig.DEFAULT_RULE_API_VERSION, extension.ruleApiVersion.get())
        assertEquals(BuildConfig.DEFAULT_RULE_API_VERSION, extension.sdkVersion.get())
    }

    @Test
    fun `krit-rule-api is added to implementation`() {
        val project = newProject()
        val deps = project.configurations.getByName("implementation").dependencies
        // Trigger lazy resolution
        val resolved = deps.toList()
        assertTrue(
            resolved.any { it.group == "dev.jasonpearson.krit" && it.name == "krit-rule-api" },
            "krit-rule-api should be added to implementation: $resolved"
        )
    }

    @Test
    fun `kritRuleJar task is registered with manifest attributes`() {
        val project = newProject()
        val task = project.tasks.findByName("kritRuleJar")
        assertNotNull(task, "kritRuleJar task should be registered")
        assertTrue(task is Jar, "kritRuleJar should be a Jar task")
        val jar = task as Jar
        assertEquals(BuildConfig.DEFAULT_RULE_API_VERSION, manifestValue(jar, "Krit-SDK-Version"))
        assertEquals("custom", manifestValue(jar, "Krit-Vendor-Id"))
        assertEquals("warning", manifestValue(jar, "Krit-Default-Severity"))
        assertEquals(BuildConfig.PLUGIN_VERSION, manifestValue(jar, "Krit-Plugin-Version"))
    }

    @Test
    fun `generateKritRuleServices task is registered`() {
        val project = newProject()
        val task = project.tasks.findByName("generateKritRuleServices")
        assertNotNull(task, "generateKritRuleServices task should be registered")
        assertTrue(task is KritRuleServicesTask)
    }

    @Test
    fun `extension overrides flow into manifest attributes`() {
        val project = newProject()
        val extension = project.extensions.getByType(KritCustomRulesExtension::class.java)
        extension.vendorId.set("acme")
        extension.defaultSeverity.set("error")
        extension.sdkVersion.set("9.9.9")

        val jar = project.tasks.getByName("kritRuleJar") as Jar
        assertEquals("9.9.9", manifestValue(jar, "Krit-SDK-Version"))
        assertEquals("acme", manifestValue(jar, "Krit-Vendor-Id"))
        assertEquals("error", manifestValue(jar, "Krit-Default-Severity"))
    }

    private fun manifestValue(jar: Jar, key: String): String? {
        val raw = jar.manifest.attributes[key] ?: return null
        return if (raw is Provider<*>) raw.get()?.toString() else raw.toString()
    }
}
