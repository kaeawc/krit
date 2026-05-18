package dev.jasonpearson.krit.gradle

import org.gradle.api.Project
import org.gradle.api.attributes.Category
import org.gradle.testfixtures.ProjectBuilder
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import dev.jasonpearson.krit.gradle.KritCheckTask.Companion.appendCustomRuleJarArgs

class KritPluginTest {

    @TempDir
    lateinit var projectDir: File

    private fun newProject(): Project {
        val project = ProjectBuilder.builder()
            .withProjectDir(projectDir)
            .build()
        project.pluginManager.apply(KritPlugin::class.java)
        return project
    }

    @Test
    fun `plugin can be applied to a project`() {
        val project = newProject()

        val extension = project.extensions.findByName("krit")
        assertNotNull(extension, "krit extension should be registered")
    }

    @Test
    fun `kritCheck task is registered`() {
        val project = newProject()

        val task = project.tasks.findByName("kritCheck")
        assertNotNull(task, "kritCheck task should be registered")
        assertTrue(task is KritCheckTask, "kritCheck should be a KritCheckTask")
    }

    @Test
    fun `kritFormat task is registered`() {
        val project = newProject()

        val task = project.tasks.findByName("kritFormat")
        assertNotNull(task, "kritFormat task should be registered")
        assertTrue(task is KritFormatTask, "kritFormat should be a KritFormatTask")
    }

    @Test
    fun `kritBaseline task is registered`() {
        val project = newProject()

        val task = project.tasks.findByName("kritBaseline")
        assertNotNull(task, "kritBaseline task should be registered")
        assertTrue(task is KritBaselineTask, "kritBaseline should be a KritBaselineTask")
    }

    @Test
    fun `top-level extension defaults are minimal and focused on user-facing knobs`() {
        val project = newProject()

        val extension = project.extensions.getByType(KritExtension::class.java)
        assertEquals(false, extension.ignoreFailures.get())
        // config, baseline, customRuleJars are unset by default (user-provided or
        // populated via the kritCustomRules configuration).
        assertFalse(extension.config.isPresent, "config should default unset for auto-discovery")
        assertFalse(extension.baseline.isPresent, "baseline should default unset")
    }

    @Test
    fun `ignoreFailures is configurable at the top level`() {
        val project = newProject()
        val extension = project.extensions.getByType(KritExtension::class.java)

        extension.ignoreFailures.set(true)
        assertEquals(true, extension.ignoreFailures.get())
    }

    @Test
    fun `advanced extension exposes escape-hatch defaults`() {
        val project = newProject()

        val advanced = project.extensions.getByType(KritExtension::class.java).advanced
        assertEquals(KritPlugin.KRIT_DEFAULT_VERSION, advanced.toolVersion.get())
        assertEquals(false, advanced.allRules.get())
        assertEquals("idiomatic", advanced.fixLevel.get())
        assertEquals(true, advanced.typeInference.get())
        assertEquals(false, advanced.noCache.get())
        assertEquals(Runtime.getRuntime().availableProcessors(), advanced.parallel.get())
    }

    @Test
    fun `advanced extension is configurable`() {
        val project = newProject()
        val advanced = project.extensions.getByType(KritExtension::class.java).advanced

        advanced.toolVersion.set("1.0.0")
        advanced.allRules.set(true)
        advanced.fixLevel.set("semantic")
        advanced.parallel.set(4)
        advanced.noCache.set(true)
        advanced.typeInference.set(false)

        assertEquals("1.0.0", advanced.toolVersion.get())
        assertEquals(true, advanced.allRules.get())
        assertEquals("semantic", advanced.fixLevel.get())
        assertEquals(4, advanced.parallel.get())
        assertEquals(true, advanced.noCache.get())
        assertEquals(false, advanced.typeInference.get())
    }

    @Test
    fun `advanced block is configurable via action`() {
        val project = newProject()
        val extension = project.extensions.getByType(KritExtension::class.java)

        extension.advanced {
            parallel.set(7)
            noCache.set(true)
        }

        assertEquals(7, extension.advanced.parallel.get())
        assertEquals(true, extension.advanced.noCache.get())
    }

    @Test
    fun `advanced settings flow through to task conventions`() {
        val project = newProject()
        val extension = project.extensions.getByType(KritExtension::class.java)

        extension.advanced.parallel.set(11)
        extension.advanced.allRules.set(true)
        extension.advanced.fixLevel.set("semantic")

        val check = project.tasks.getByName("kritCheck") as KritCheckTask
        val fmt = project.tasks.getByName("kritFormat") as KritFormatTask
        val baseline = project.tasks.getByName("kritBaseline") as KritBaselineTask

        assertEquals(11, check.parallel.get())
        assertEquals(true, check.allRules.get())
        assertEquals("semantic", fmt.fixLevel.get())
        assertEquals(true, baseline.allRules.get())
    }

    @Test
    fun `KritExtension exposes only the supported top-level surface`() {
        // KritExtension is the public DSL surface. Escape hatches live under
        // `advanced { }`; custom-rule wiring goes through the `kritCustomRules`
        // dependency configuration or the `customRuleJars` file collection.
        // Each entry here pins one accessor (or DSL method name) that must NOT
        // appear on KritExtension. The check runs via reflection so the test
        // does not need to import the moved Property<*> APIs.
        val forbidden = mapOf(
            "getToolVersion" to "lives under `advanced`",
            "getAllRules" to "lives under `advanced`",
            "getFixLevel" to "lives under `advanced`",
            "getParallel" to "lives under `advanced`",
            "getNoCache" to "lives under `advanced`",
            "getTypeInference" to "lives under `advanced`",
            "getBinary" to "lives under `advanced`",
            "getReportsDir" to "lives under `advanced`",
            "getSource" to "lives under `advanced`",
            "customRules" to "use the `kritCustomRules` dependency " +
                "configuration or `customRuleJars.from(...)`",
        )
        val publicMembers = KritExtension::class.java.methods.map { it.name }.toSet()
        forbidden.forEach { (name, guidance) ->
            assertFalse(name in publicMembers,
                "KritExtension exposes $name; $guidance")
        }
    }

    @Test
    fun `customRuleJars accepts raw file notations`() {
        val project = newProject()

        val extension = project.extensions.getByType(KritExtension::class.java)
        val rulesJar = project.file("build-logic/krit-rules/build/libs/krit-rules.jar")

        extension.customRuleJars.from(rulesJar)

        assertTrue(extension.customRuleJars.files.contains(rulesJar))
    }

    @Test
    fun `kritCheck task receives custom rule jars from extension`() {
        val project = newProject()

        val extension = project.extensions.getByType(KritExtension::class.java)
        val task = project.tasks.getByName("kritCheck") as KritCheckTask
        val rulesJar = project.file("build-logic/krit-rules/build/libs/krit-rules.jar")

        extension.customRuleJars.from(rulesJar)

        assertTrue(task.customRuleJars.files.contains(rulesJar))
    }

    @Test
    fun `kritBaseline task receives custom rule jars from extension`() {
        val project = newProject()

        val extension = project.extensions.getByType(KritExtension::class.java)
        val task = project.tasks.getByName("kritBaseline") as KritBaselineTask
        val rulesJar = project.file("build-logic/krit-rules/build/libs/krit-rules.jar")

        extension.customRuleJars.from(rulesJar)

        assertTrue(task.customRuleJars.files.contains(rulesJar),
            "kritBaseline must inherit customRuleJars so the baseline captures plugin findings")
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
        val project = newProject()

        val task = project.tasks.getByName("kritFormat") as KritFormatTask
        assertEquals("Apply krit auto-fixes to Kotlin sources", task.description)
        assertEquals("build", task.group)
    }

    @Test
    fun `kritBaseline task has correct description and group`() {
        val project = newProject()

        val task = project.tasks.getByName("kritBaseline") as KritBaselineTask
        assertEquals("Create a krit baseline file from current findings", task.description)
        assertEquals("build", task.group)
    }

    @Test
    fun `kritBaseline task has default output file`() {
        val project = newProject()

        val task = project.tasks.getByName("kritBaseline") as KritBaselineTask
        val baselinePath = task.baselineFile.get().asFile.absolutePath
        assertTrue(baselinePath.endsWith("reports/krit/baseline.xml"),
            "Baseline file should default to reports/krit/baseline.xml, was: $baselinePath")
    }

    // --- Reports DSL tests ---

    @Test
    fun `reports DSL has correct defaults`() {
        val project = newProject()

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
        val project = newProject()

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
        val project = newProject()

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
        val project = newProject()

        val extension = project.extensions.getByType(KritExtension::class.java)

        extension.reports {
            sarif.required.set(false)
            json.required.set(true)
            checkstyle.required.set(true)
        }

        assertFalse(extension.reports.sarif.required.get())
        assertTrue(extension.reports.json.required.get())
        assertTrue(extension.reports.checkstyle.required.get())
    }

    @Test
    fun `kritCheck task receives report conventions from extension`() {
        val project = newProject()

        val task = project.tasks.getByName("kritCheck") as KritCheckTask

        // The task should have report properties wired from the extension
        assertTrue(task.sarifRequired.get(), "Task SARIF required should default to true")
        assertFalse(task.jsonRequired.get(), "Task JSON required should default to false")
        assertFalse(task.plainRequired.get(), "Task plain required should default to false")
        assertFalse(task.checkstyleRequired.get(), "Task checkstyle required should default to false")
    }

    @Test
    fun `kritCheck task report conventions follow extension changes`() {
        val project = newProject()

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
        val project = newProject()

        // Without Kotlin JVM plugin, only the aggregate kritCheck should exist
        assertNotNull(project.tasks.findByName("kritCheck"))
        // No per-source-set tasks
        assertEquals(null, project.tasks.findByName("kritCheckMain"))
        assertEquals(null, project.tasks.findByName("kritCheckTest"))
    }

    @Test
    fun `per-variant tasks are not registered without android plugin`() {
        val project = newProject()

        // Without Android plugin, no variant-specific tasks
        assertEquals(null, project.tasks.findByName("kritCheckDebug"))
        assertEquals(null, project.tasks.findByName("kritCheckRelease"))
    }

    // --- Custom rule jars + --daemon auto-pass ---

    @Test
    fun `appendCustomRuleJarArgs is a no-op when jar list is empty`() {
        val args = mutableListOf<String>()
        args.appendCustomRuleJarArgs(emptyList())
        assertTrue(args.isEmpty(), "empty jar list should not mutate args, was: $args")
    }

    @Test
    fun `appendCustomRuleJarArgs adds --daemon automatically when jars present`() {
        val args = mutableListOf("--format=sarif")
        args.appendCustomRuleJarArgs(listOf("/jars/a.jar", "/jars/b.jar"))
        val index = args.indexOf("--custom-rule-jars")
        assertTrue(index >= 0, "expected --custom-rule-jars, was: $args")
        assertEquals("/jars/a.jar,/jars/b.jar", args[index + 1])
        assertTrue("--daemon" in args,
            "expected --daemon to be auto-passed when custom-rule jars configured, was: $args")
    }

    @Test
    fun `appendCustomRuleJarArgs does not duplicate --daemon when already present`() {
        val args = mutableListOf("--daemon", "--format=sarif")
        args.appendCustomRuleJarArgs(listOf("/jars/a.jar"))
        assertEquals(1, args.count { it == "--daemon" },
            "--daemon should not be duplicated, was: $args")
    }

    // --- kritCustomRules resolvable configuration (variant-aware wiring) ---

    @Test
    fun `kritCustomRules configuration is registered with krit-rule-bundle category`() {
        val project = newProject()
        val config = project.configurations.findByName("kritCustomRules")
        assertNotNull(config, "kritCustomRules configuration should be registered")
        assertFalse(config!!.isCanBeConsumed,
            "kritCustomRules should not be consumable — it's a resolver, not a publisher")
        assertTrue(config.isCanBeResolved,
            "kritCustomRules should be resolvable so it can read variant artifacts")
        val category = config.attributes.getAttribute(Category.CATEGORY_ATTRIBUTE)
        assertNotNull(category, "kritCustomRules must declare a Category attribute")
        assertEquals(KritPlugin.KRIT_RULE_BUNDLE_CATEGORY, category!!.name)
    }

    @Test
    fun `files added to kritCustomRules flow into extension customRuleJars`() {
        val project = newProject()
        val fakeJar = File(projectDir, "fake.jar").apply { writeText("not a real jar") }

        project.dependencies.add("kritCustomRules", project.files(fakeJar))

        val extension = project.extensions.getByType(KritExtension::class.java)
        assertTrue(extension.customRuleJars.files.contains(fakeJar),
            "files declared on the kritCustomRules dep config must appear in customRuleJars")

        val task = project.tasks.getByName("kritCheck") as KritCheckTask
        assertTrue(task.customRuleJars.files.contains(fakeJar),
            "kritCheck task must inherit the resolved jars too")
    }

}
