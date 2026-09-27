package dev.jasonpearson.krit.gradle

import groovy.json.JsonSlurper
import org.gradle.testkit.runner.GradleRunner
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.util.jar.JarOutputStream

class KritExportModelTest {
    @TempDir lateinit var directory: File

    private fun fixture(): File {
        val root = File(directory, "fixture").also { it.mkdirs() }
        val artifact = File(root, "repo/com/example/testlib/1.0/testlib-1.0.jar")
        artifact.parentFile.mkdirs()
        JarOutputStream(artifact.outputStream()).use { }
        artifact.resolveSibling("testlib-1.0.pom").writeText("""
            <project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId>
            <artifactId>testlib</artifactId><version>1.0</version></project>
        """.trimIndent())
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"fixture\"\ninclude(\"app\", \"lib\", \"plain\")\n")
        File(root, "build.gradle.kts").writeText("plugins { id(\"dev.jasonpearson.krit\") }\n")
        File(root, "app").mkdirs()
        File(root, "app/build.gradle.kts").writeText("""
            plugins { `java-library`; id("dev.jasonpearson.krit") }
            repositories { maven { url = uri(rootProject.file("repo")) } }
            dependencies { implementation(project(":lib")); implementation("com.example:testlib:1.0") }
        """.trimIndent())
        File(root, "lib").mkdirs()
        File(root, "lib/build.gradle.kts").writeText("plugins { `java-library`; id(\"dev.jasonpearson.krit\") }\n")
        File(root, "plain").mkdirs()
        File(root, "plain/build.gradle.kts").writeText("plugins { id(\"dev.jasonpearson.krit\") }\n")
        return root
    }

    private fun run(root: File, vararg args: String) = GradleRunner.create()
        .withProjectDir(root)
        .withPluginClasspath()
        .withArguments(*args, "--offline", "--stacktrace")
        .build()

    private fun runWithFakeAndroid(root: File, vararg args: String): org.gradle.testkit.runner.BuildResult {
        val runner = GradleRunner.create().withPluginClasspath()
        val classes = File(FakeAndroidPlugin::class.java.protectionDomain.codeSource.location.toURI())
        val descriptor = File(
            requireNotNull(FakeAndroidPlugin::class.java.classLoader.getResource(
                "META-INF/gradle-plugins/com.android.library.properties"
            )).toURI()
        )
        val resources = descriptor.parentFile.parentFile.parentFile
        return runner.withProjectDir(root)
            .withPluginClasspath(runner.pluginClasspath + listOf(classes, resources))
            .withArguments(*args, "--offline", "--stacktrace")
            .build()
    }

    @Test
    fun `per project exports external artifacts without compiling project dependencies`() {
        val root = fixture()
        val result = run(root, "kritExportModel")
        val json = File(root, ".krit/gradle-model/app.json").readText()
        println("APP MODEL JSON:\n$json")
        @Suppress("UNCHECKED_CAST")
        val projects = (JsonSlurper().parseText(json) as Map<String, Any>)["projects"] as List<Map<String, Any>>
        assertEquals(listOf(":app"), projects.map { it["path"] })
        assertTrue(File(root, ".krit/gradle-model/lib.json").isFile)
        assertFalse(File(root, ".krit/gradle-model/plain.json").exists())
        assertFalse(File(root, ".krit/gradle-model/_root.json").exists())
        @Suppress("UNCHECKED_CAST")
        val appSources = projects.first()["sourceSets"] as List<Map<String, Any>>
        @Suppress("UNCHECKED_CAST")
        val classpath = appSources.single()["classpath"] as List<String>
        assertTrue(classpath.any { it.endsWith("testlib-1.0.jar") }, classpath.toString())
        assertFalse(classpath.any { it.contains("/lib/build/") })
        assertEquals(listOf(":lib"), appSources.single()["projectDeps"])
        assertFalse(result.tasks.any { it.path in listOf(":lib:compileJava", ":lib:jar", ":lib:classes") })
        assertFalse(result.tasks.any { it.path == ":plain:kritExportModel" })

    }

    @Test
    fun `configuration cache is reused`() {
        val root = fixture()
        run(root, "kritExportModel", "--configuration-cache")
        val before = File(root, ".krit/gradle-model/app.json").readBytes()
        val second = run(root, "kritExportModel", "--configuration-cache")
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        assertTrue(before.contentEquals(File(root, ".krit/gradle-model/app.json").readBytes()))
    }

    @Test
    fun `project without language plugin has no export task`() {
        val root = fixture()
        val result = run(root, ":plain:tasks", "--all")
        assertFalse(result.output.contains("kritExportModel"), result.output)
    }

    @Test
    fun `android selection and boot paths are deterministic`() {
        assertEquals("debug", pickAndroidVariant(listOf("release", "debug"), "debug"))
        assertEquals("debug", pickAndroidVariant(listOf("release", "debug"), "missing"))
        assertEquals("debug", pickAndroidVariant(listOf("release", "debug", "staging"), "missing"))
        assertEquals(null, pickAndroidVariant(emptyList(), "debug"))
        val jar = File(directory, "android.jar")
        assertEquals(listOf(modelPath(jar)), bootClasspathPaths(listOf(jar, jar)))
    }

    @Test
    fun `subproject tasks export with configuration cache and isolated projects`() {
        val root = File(directory, "topology").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"topology\"\ninclude(\"one\", \"two\")\n")
        File(root, "build.gradle.kts").writeText("")
        listOf("one", "two").forEach { name ->
            File(root, name).mkdirs()
            File(root, "$name/build.gradle.kts").writeText("plugins { `java-library`; id(\"dev.jasonpearson.krit\") }\n")
        }
        run(root, "kritExportModel", "--configuration-cache")
        assertTrue(File(root, ".krit/gradle-model/one.json").isFile)
        assertTrue(File(root, ".krit/gradle-model/two.json").isFile)
        assertFalse(File(root, ".krit/gradle-model/_root.json").exists())
        val second = run(root, "kritExportModel", "--configuration-cache")
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        val isolated = try {
            run(root, "kritExportModel", "--configuration-cache", "-Dorg.gradle.unsafe.isolated-projects=true")
        } catch (error: org.gradle.testkit.runner.UnexpectedBuildFailure) {
            if (error.message.orEmpty().contains("isolated projects", ignoreCase = true) &&
                error.message.orEmpty().contains("not supported", ignoreCase = true)) null else throw error
        }
        if (isolated != null) {
            assertTrue(isolated.tasks.any { it.path == ":one:kritExportModel" })
            assertTrue(isolated.tasks.any { it.path == ":two:kritExportModel" })
        }
    }

    @Test
    fun `android DSL and configuration names export model and missing components warns`() {
        val root = File(directory, "android").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"android\"\ninclude(\"app\")\n")
        File(root, "build.gradle.kts").writeText("")
        File(root, "app").mkdirs()
        File(root, "app/build.gradle.kts").writeText("plugins { id(\"com.android.library\"); id(\"dev.jasonpearson.krit\") }\n")
        File(root, "app/android.jar").writeBytes(byteArrayOf())
        runWithFakeAndroid(root, "kritExportModel")
        val model = File(root, ".krit/gradle-model/app.json")
        println("ANDROID MODEL JSON:\n${model.readText()}")
        @Suppress("UNCHECKED_CAST")
        val projects = (JsonSlurper().parse(model) as Map<String, Any>)["projects"] as List<Map<String, Any>>
        @Suppress("UNCHECKED_CAST")
        val sources = projects.single()["sourceSets"] as List<Map<String, Any>>
        assertEquals("debug", sources.single()["variant"])
        assertTrue((sources.single()["sourceDirs"] as List<*>).any { it.toString().endsWith("src/debug/java") })
        assertTrue((sources.single()["bootClasspath"] as List<*>).any { it.toString().endsWith("android.jar") })
        val missing = runWithFakeAndroid(root, "kritExportModel", "-PomitComponents", "--rerun-tasks")
        assertTrue(missing.output.contains("kritExportModel skipped Android model for :app"), missing.output)
        assertTrue(model.isFile)
    }

    @Test
    fun `missing optional flavored variant source set preserves Android model`() {
        val root = File(directory, "flavored-android").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"flavored-android\"\ninclude(\"app\")\n")
        File(root, "build.gradle.kts").writeText("")
        File(root, "app").mkdirs()
        File(root, "app/build.gradle.kts").writeText("""
            plugins { id("com.android.library"); id("dev.jasonpearson.krit") }
            krit { androidVariant = "stagingDebug" }
        """.trimIndent())
        File(root, "app/android.jar").writeBytes(byteArrayOf())

        runWithFakeAndroid(root, "kritExportModel")
        val model = File(root, ".krit/gradle-model/app.json")
        @Suppress("UNCHECKED_CAST")
        val projects = (JsonSlurper().parse(model) as Map<String, Any>)["projects"] as List<Map<String, Any>>
        @Suppress("UNCHECKED_CAST")
        val sourceSets = projects.single()["sourceSets"] as List<Map<String, Any>>
        val sourceSet = sourceSets.single()
        assertEquals("stagingDebug", sourceSet["variant"])
        assertTrue(sourceSet.containsKey("classpath"), sourceSet.toString())
        @Suppress("UNCHECKED_CAST")
        val bootClasspath = sourceSet["bootClasspath"] as List<String>
        assertTrue(bootClasspath.any { it.endsWith("android.jar") }, bootClasspath.toString())
        @Suppress("UNCHECKED_CAST")
        val sourceDirs = sourceSet["sourceDirs"] as List<String>
        for (name in listOf("main", "debug")) {
            for (language in listOf("java", "kotlin")) {
                assertTrue(sourceDirs.any { it.endsWith("src/$name/$language") }, sourceDirs.toString())
            }
        }
        assertFalse(sourceDirs.any { it.contains("src/stagingDebug/") }, sourceDirs.toString())
    }
}
