package dev.krit.gradle

import org.gradle.testfixtures.ProjectBuilder
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

class KritPluginTest {

    @TempDir
    lateinit var projectDir: File

    @Test
    fun `plugin can be applied to a project`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.findByName("krit")
        assertNotNull(extension, "krit extension should be registered")
    }

    @Test
    fun `kritCheck task is registered`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.findByName("kritCheck")
        assertNotNull(task, "kritCheck task should be registered")
        assertTrue(task is KritCheckTask, "kritCheck should be a KritCheckTask")
    }

    @Test
    fun `kritFormat task is registered`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.findByName("kritFormat")
        assertNotNull(task, "kritFormat task should be registered")
        assertTrue(task is KritFormatTask, "kritFormat should be a KritFormatTask")
    }

    @Test
    fun `kritBaseline task is registered`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.findByName("kritBaseline")
        assertNotNull(task, "kritBaseline task should be registered")
        assertTrue(task is KritBaselineTask, "kritBaseline should be a KritBaselineTask")
    }

    @Test
    fun `extension has correct default values`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)
        assertEquals(KritPlugin.KRIT_DEFAULT_VERSION, extension.toolVersion.get())
        assertEquals(false, extension.allRules.get())
        assertEquals(false, extension.ignoreFailures.get())
        assertEquals("idiomatic", extension.fixLevel.get())
        assertEquals(true, extension.typeInference.get())
        assertEquals(false, extension.noCache.get())
        assertEquals(Runtime.getRuntime().availableProcessors(), extension.parallel.get())
    }

    @Test
    fun `extension properties are configurable`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)

        extension.toolVersion.set("1.0.0")
        assertEquals("1.0.0", extension.toolVersion.get())

        extension.allRules.set(true)
        assertEquals(true, extension.allRules.get())

        extension.ignoreFailures.set(true)
        assertEquals(true, extension.ignoreFailures.get())

        extension.fixLevel.set("semantic")
        assertEquals("semantic", extension.fixLevel.get())

        extension.parallel.set(4)
        assertEquals(4, extension.parallel.get())

        extension.noCache.set(true)
        assertEquals(true, extension.noCache.get())

        extension.typeInference.set(false)
        assertEquals(false, extension.typeInference.get())
    }

    @Test
    fun `platform detection returns valid platform`() {
        val platform = KritBinaryResolver.detectPlatform()
        assertTrue(platform.os in listOf("darwin", "linux", "windows"))
        assertTrue(platform.arch in listOf("amd64", "arm64"))
        assertTrue(platform.binaryName.startsWith("krit"))
        assertTrue(platform.archiveName.endsWith(".tar.gz"))
    }

    @Test
    fun `platform binary name is correct for each OS`() {
        val darwinPlatform = KritBinaryResolver.Platform("darwin", "arm64")
        assertEquals("krit", darwinPlatform.binaryName)
        assertEquals("krit-darwin-arm64.tar.gz", darwinPlatform.archiveName)

        val linuxPlatform = KritBinaryResolver.Platform("linux", "amd64")
        assertEquals("krit", linuxPlatform.binaryName)
        assertEquals("krit-linux-amd64.tar.gz", linuxPlatform.archiveName)

        val windowsPlatform = KritBinaryResolver.Platform("windows", "amd64")
        assertEquals("krit.exe", windowsPlatform.binaryName)
        assertEquals("krit-windows-amd64.tar.gz", windowsPlatform.archiveName)
    }

    @Test
    fun `kritFormat task has correct description and group`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.getByName("kritFormat") as KritFormatTask
        assertEquals("Apply krit auto-fixes to Kotlin sources", task.description)
        assertEquals("build", task.group)
    }

    @Test
    fun `kritBaseline task has correct description and group`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.getByName("kritBaseline") as KritBaselineTask
        assertEquals("Create a krit baseline file from current findings", task.description)
        assertEquals("build", task.group)
    }

    @Test
    fun `kritBaseline task has default output file`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.getByName("kritBaseline") as KritBaselineTask
        val baselinePath = task.baselineFile.get().asFile.absolutePath
        assertTrue(baselinePath.endsWith("reports/krit/baseline.xml"),
            "Baseline file should default to reports/krit/baseline.xml, was: $baselinePath")
    }

    // --- Reports DSL tests ---

    @Test
    fun `reports DSL has correct defaults`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)

        // SARIF is enabled by default
        assertTrue(extension.reports.sarif.required.get(),
            "SARIF report should be enabled by default")
        // Other formats are disabled by default
        assertFalse(extension.reports.json.required.get(),
            "JSON report should be disabled by default")
        assertFalse(extension.reports.plain.required.get(),
            "Plain report should be disabled by default")
        assertFalse(extension.reports.checkstyle.required.get(),
            "Checkstyle report should be disabled by default")
    }

    @Test
    fun `reports DSL has correct default output locations`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)

        val sarifPath = extension.reports.sarif.outputLocation.get().asFile.absolutePath
        assertTrue(sarifPath.endsWith("reports/krit/krit.sarif"),
            "SARIF default path should end with reports/krit/krit.sarif, was: $sarifPath")

        val jsonPath = extension.reports.json.outputLocation.get().asFile.absolutePath
        assertTrue(jsonPath.endsWith("reports/krit/krit.json"),
            "JSON default path should end with reports/krit/krit.json, was: $jsonPath")

        val plainPath = extension.reports.plain.outputLocation.get().asFile.absolutePath
        assertTrue(plainPath.endsWith("reports/krit/krit.txt"),
            "Plain default path should end with reports/krit/krit.txt, was: $plainPath")

        val checkstylePath = extension.reports.checkstyle.outputLocation.get().asFile.absolutePath
        assertTrue(checkstylePath.endsWith("reports/krit/krit-checkstyle.xml"),
            "Checkstyle default path should end with reports/krit/krit-checkstyle.xml, was: $checkstylePath")
    }

    @Test
    fun `reports DSL is configurable`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)

        // Disable SARIF, enable JSON
        extension.reports.sarif.required.set(false)
        extension.reports.json.required.set(true)
        extension.reports.json.outputLocation.set(project.file("custom/output.json"))

        assertFalse(extension.reports.sarif.required.get())
        assertTrue(extension.reports.json.required.get())
        assertTrue(extension.reports.json.outputLocation.get().asFile.absolutePath.endsWith("custom/output.json"))
    }

    @Test
    fun `reports DSL configurable via action block`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)

        extension.reports {
            it.sarif.required.set(false)
            it.json.required.set(true)
            it.checkstyle.required.set(true)
        }

        assertFalse(extension.reports.sarif.required.get())
        assertTrue(extension.reports.json.required.get())
        assertTrue(extension.reports.checkstyle.required.get())
    }

    @Test
    fun `kritCheck task receives report conventions from extension`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val task = project.tasks.getByName("kritCheck") as KritCheckTask

        // The task should have report properties wired from the extension
        assertTrue(task.sarifRequired.get(), "Task SARIF required should default to true")
        assertFalse(task.jsonRequired.get(), "Task JSON required should default to false")
        assertFalse(task.plainRequired.get(), "Task plain required should default to false")
        assertFalse(task.checkstyleRequired.get(), "Task checkstyle required should default to false")
    }

    @Test
    fun `kritCheck task report conventions follow extension changes`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        val extension = project.extensions.getByType(KritExtension::class.java)
        extension.reports.json.required.set(true)
        extension.reports.sarif.required.set(false)

        val task = project.tasks.getByName("kritCheck") as KritCheckTask
        assertFalse(task.sarifRequired.get(), "Task SARIF should reflect extension change")
        assertTrue(task.jsonRequired.get(), "Task JSON should reflect extension change")
    }

    // --- Per-source-set task registration tests ---

    @Test
    fun `per-source-set tasks are not registered without kotlin jvm plugin`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        // Without Kotlin JVM plugin, only the aggregate kritCheck should exist
        assertNotNull(project.tasks.findByName("kritCheck"))
        // No per-source-set tasks
        assertEquals(null, project.tasks.findByName("kritCheckMain"))
        assertEquals(null, project.tasks.findByName("kritCheckTest"))
    }

    @Test
    fun `per-variant tasks are not registered without android plugin`() {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()

        project.plugins

        // Without Android plugin, no variant-specific tasks
        assertEquals(null, project.tasks.findByName("kritCheckDebug"))
        assertEquals(null, project.tasks.findByName("kritCheckRelease"))
    }
}
