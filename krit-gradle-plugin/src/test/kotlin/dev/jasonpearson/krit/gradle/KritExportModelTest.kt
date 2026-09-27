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

    private fun runWithFakePlugin(root: File, pluginClass: Class<*>, descriptorName: String,
                                  vararg args: String): org.gradle.testkit.runner.BuildResult {
        val runner = GradleRunner.create().withPluginClasspath()
        val classes = File(pluginClass.protectionDomain.codeSource.location.toURI())
        val descriptor = File(
            requireNotNull(pluginClass.classLoader.getResource(
                "META-INF/gradle-plugins/$descriptorName.properties"
            )).toURI()
        )
        val resources = descriptor.parentFile.parentFile.parentFile
        return runner.withProjectDir(root)
            .withPluginClasspath(runner.pluginClasspath + listOf(classes, resources))
            .withArguments(*args, "--offline", "--stacktrace")
            .build()
    }

    private fun runWithFakeAndroid(root: File, vararg args: String) =
        runWithFakePlugin(root, FakeAndroidPlugin::class.java, "com.android.library", *args)

    private fun sourceSets(model: File): List<Map<String, Any>> {
        @Suppress("UNCHECKED_CAST")
        val projects = (JsonSlurper().parse(model) as Map<String, Any>)["projects"] as List<Map<String, Any>>
        @Suppress("UNCHECKED_CAST")
        return projects.single()["sourceSets"] as List<Map<String, Any>>
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
        val main = appSources.single { it["kind"] == "main" }
        val classpath = main["classpath"] as List<String>
        assertTrue(classpath.any { it.endsWith("testlib-1.0.jar") }, classpath.toString())
        assertFalse(classpath.any { it.contains("/lib/build/") })
        assertEquals(listOf(":lib"), main["projectDeps"])
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
        val main = sources.single { it["kind"] == "main" }
        assertEquals("debug", main["variant"])
        assertTrue((main["sourceDirs"] as List<*>).any { it.toString().endsWith("src/debug/java") })
        assertTrue((main["bootClasspath"] as List<*>).any { it.toString().endsWith("android.jar") })
        assertTrue((main["generatedClasspath"] as List<*>).any {
            it.toString().endsWith("build/intermediates/compile_r_class_jar/debug/generateDebugRFile/R.jar")
        })
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
        val sourceSet = sourceSets.single { it["kind"] == "main" }
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
        for (kind in listOf("test", "androidTest")) {
            @Suppress("UNCHECKED_CAST")
            val testDirs = sourceSets.single { it["kind"] == kind }["sourceDirs"] as List<String>
            assertTrue(testDirs.any { it.endsWith("src/${kind}Staging/java") }, testDirs.toString())
            assertTrue(testDirs.any { it.endsWith("src/${kind}Debug/java") }, testDirs.toString())
        }
    }

    @Test
    fun `java test fixtures export three kinds and distinct classpaths`() {
        val root = fixture()
        File(root, "app/test-only.jar").writeBytes(byteArrayOf())
        File(root, "app/fixture-only.jar").writeBytes(byteArrayOf())
        File(root, "app/build.gradle.kts").appendText("""

            apply(plugin = "java-test-fixtures")
            dependencies {
                testImplementation(files("test-only.jar"))
                add("testFixturesImplementation", files("fixture-only.jar"))
            }
        """.trimIndent())
        run(root, ":app:kritExportModel")
        val sets = sourceSets(File(root, ".krit/gradle-model/app.json"))
        assertEquals(setOf("main", "test", "testFixtures"), sets.map { it["kind"] }.toSet())
        @Suppress("UNCHECKED_CAST")
        val testPath = sets.single { it["kind"] == "test" }["classpath"] as List<String>
        @Suppress("UNCHECKED_CAST")
        val fixturesPath = sets.single { it["kind"] == "testFixtures" }["classpath"] as List<String>
        assertTrue(testPath.any { it.endsWith("test-only.jar") }, testPath.toString())
        assertTrue(fixturesPath.any { it.endsWith("fixture-only.jar") }, fixturesPath.toString())
    }

    @Test
    fun `build directory sources are partitioned as generated`() {
        val root = fixture()
        File(root, "app/build.gradle.kts").appendText("\nsourceSets.main { java.srcDir(layout.buildDirectory.dir(\"generated/source/example\")) }\n")
        run(root, ":app:kritExportModel")
        val main = sourceSets(File(root, ".krit/gradle-model/app.json")).single { it["kind"] == "main" }
        @Suppress("UNCHECKED_CAST")
        val ordinary = main["sourceDirs"] as List<String>
        @Suppress("UNCHECKED_CAST")
        val generated = main["generatedSourceDirs"] as List<String>
        assertTrue(generated.any { it.endsWith("build/generated/source/example") }, generated.toString())
        assertFalse(ordinary.any { it.endsWith("build/generated/source/example") })
        assertFalse(File(root, "app/build/generated/source/example").exists())
    }

    @Test
    fun `KMP JVM compilations export and JS is omitted`() {
        val root = File(directory, "kmp").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"kmp\"\n")
        File(root, "build.gradle.kts").writeText("plugins { id(\"org.jetbrains.kotlin.multiplatform\"); id(\"com.google.devtools.ksp\"); id(\"dev.jasonpearson.krit\") }\nkrit { exportGenerated = true }\n")
        runWithFakePlugin(root, FakeKmpPlugin::class.java, "org.jetbrains.kotlin.multiplatform",
            "kritExportModel", "--configuration-cache")
        val cached = runWithFakePlugin(root, FakeKmpPlugin::class.java, "org.jetbrains.kotlin.multiplatform",
            "kritExportModel", "--configuration-cache")
        assertTrue(cached.output.contains("Reusing configuration cache."), cached.output)
        val sets = sourceSets(File(root, ".krit/gradle-model/_root.json"))
        assertEquals(setOf("jvmMain", "jvmTest"), sets.map { it["name"] }.toSet())
        assertEquals(setOf("main", "test"), sets.map { it["kind"] }.toSet())
        val main = sets.single { it["name"] == "jvmMain" }
        assertTrue((main["sourceDirs"] as List<*>).any { it.toString().endsWith("src/commonMain/kotlin") })
        assertTrue((main["sourceDirs"] as List<*>).any { it.toString().endsWith("src/jvmMain/kotlin") })
        val graph = runWithFakePlugin(root, FakeKmpPlugin::class.java,
            "org.jetbrains.kotlin.multiplatform", "kritExportModel", "--dry-run")
        assertTrue(graph.output.contains(":kspKotlinJvm SKIPPED"), graph.output)
        assertFalse(graph.output.contains(":kspDebugKotlin SKIPPED"), graph.output)
    }

    @Test
    fun `KMP Android compilations keep Android model facts without duplicate variants`() {
        val root = File(directory, "kmp-android").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"kmp-android\"\n")
        File(root, "build.gradle.kts").writeText("""
            plugins { id("com.android.library"); id("org.jetbrains.kotlin.multiplatform"); id("dev.jasonpearson.krit") }
        """.trimIndent())
        File(root, "android.jar").writeBytes(byteArrayOf())
        runWithFakePlugin(root, FakeKmpPlugin::class.java,
            "org.jetbrains.kotlin.multiplatform", "kritExportModel", "-PfakeAndroidKmp")
        val sets = sourceSets(File(root, ".krit/gradle-model/_root.json"))
        val android = sets.filter { it["platform"] == "android" }
        assertEquals(setOf("main", "test", "androidTest"), android.map { it["kind"] }.toSet())
        assertEquals(3, android.size)
        val expected = mapOf(
            "main" to "compile_r_class_jar/debug/generateDebugRFile/R.jar",
            "test" to "compile_and_runtime_not_namespaced_r_class_jar/debugUnitTest/generateDebugUnitTestStubRFile/R.jar",
            "androidTest" to "compile_and_runtime_not_namespaced_r_class_jar/debugAndroidTest/processDebugAndroidTestResources/R.jar",
        )
        android.forEach { entry ->
            assertTrue((entry["bootClasspath"] as List<*>).any { it.toString().endsWith("android.jar") })
            assertEquals("debug", entry["variant"])
            assertEquals(listOf(File(root, "build/intermediates/${expected[entry["kind"]]}").canonicalPath),
                entry["generatedClasspath"])
        }
    }

    @Test
    fun `Android library main R jar is found in task output directory`() {
        val root = File(directory, "library-r-jar").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"library-r-jar\"\n")
        File(root, "build.gradle.kts").writeText("plugins { id(\"com.android.library\"); id(\"dev.jasonpearson.krit\") }\n")
        runWithFakeAndroid(root, "kritExportModel", "-PfakeRJarKind=main")
        val sets = sourceSets(File(root, ".krit/gradle-model/_root.json"))
        val jar = File(root, "build/intermediates/compile_r_class_jar/debug/generateDebugRFile/R.jar")
        assertTrue(jar.isFile)
        assertEquals(listOf(jar.canonicalPath), sets.single { it["kind"] == "main" }["generatedClasspath"])
        val predictedTest = File(root,
            "build/intermediates/compile_and_runtime_not_namespaced_r_class_jar/debugUnitTest/generateDebugUnitTestStubRFile/R.jar")
        assertFalse(predictedTest.exists())
        assertEquals(listOf(predictedTest.canonicalPath),
            sets.single { it["kind"] == "test" }["generatedClasspath"])
    }

    @Test
    fun `Android test R jar scan collects existing task outputs from both runtime folders`() {
        val root = File(directory, "android-test-r-jar").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"android-test-r-jar\"\n")
        File(root, "build.gradle.kts").writeText("plugins { id(\"com.android.library\"); id(\"dev.jasonpearson.krit\") }\n")
        val otherJar = File(root,
            "build/intermediates/compile_and_runtime_r_class_jar/debugAndroidTest/customTask/R.jar")
        otherJar.parentFile.mkdirs()
        otherJar.writeBytes(byteArrayOf())
        runWithFakeAndroid(root, "kritExportModel", "-PfakeRJarKind=androidTest")
        val fakeJar = File(root,
            "build/intermediates/compile_and_runtime_not_namespaced_r_class_jar/debugAndroidTest/processDebugAndroidTestResources/R.jar")
        assertTrue(fakeJar.isFile)
        assertEquals(listOf(otherJar.canonicalPath, fakeJar.canonicalPath).sorted(),
            sourceSets(File(root, ".krit/gradle-model/_root.json"))
                .single { it["kind"] == "androidTest" }["generatedClasspath"])
    }

    @Test
    fun `model registration does not realize compile tasks on help`() {
        val root = File(directory, "lazy-model").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"lazy-model\"\n")
        File(root, "build.gradle.kts").writeText("""
            plugins { id("com.android.library"); id("dev.jasonpearson.krit") }
            tasks.configureEach { if (name.startsWith("compile")) println("REALIZED_COMPILE_TASK:" + name) }
        """.trimIndent())
        val result = runWithFakeAndroid(root, "help")
        assertFalse(result.output.contains("REALIZED_COMPILE_TASK:"), result.output)
    }

    @Test
    fun `Android tests and JVM target export while missing target is empty`() {
        val root = File(directory, "android-tests").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"android-tests\"\n")
        File(root, "build.gradle.kts").writeText("plugins { id(\"com.android.library\"); id(\"dev.jasonpearson.krit\") }\n")
        File(root, "android.jar").writeBytes(byteArrayOf())
        runWithFakeAndroid(root, "kritExportModel", "--configuration-cache")
        val cached = runWithFakeAndroid(root, "kritExportModel", "--configuration-cache")
        assertTrue(cached.output.contains("Reusing configuration cache."), cached.output)
        val sets = sourceSets(File(root, ".krit/gradle-model/_root.json"))
        assertEquals(setOf("main", "test", "androidTest"), sets.map { it["kind"] }.toSet())
        assertEquals("17", sets.single { it["kind"] == "main" }["jvmTarget"])
        assertEquals("", sets.single { it["kind"] == "test" }["jvmTarget"])
        assertTrue((sets.single { it["kind"] == "androidTest" }["sourceDirs"] as List<*>)
            .any { it.toString().endsWith("src/androidTest/java") })
    }

    @Test
    fun `generated task dependencies are opt in`() {
        val root = File(directory, "generated-graph").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"generated-graph\"\n")
        val build = File(root, "build.gradle.kts")
        build.writeText("plugins { id(\"com.android.library\"); id(\"com.google.devtools.ksp\"); id(\"dev.jasonpearson.krit\") }\n")
        val default = runWithFakeAndroid(root, "kritExportModel", "--dry-run")
        assertFalse(default.output.contains(":generateDebugRFile SKIPPED"))
        assertFalse(default.output.contains(":processDebugResources SKIPPED"))
        assertFalse(default.output.contains(":kspDebugKotlin SKIPPED"))
        build.appendText("\nkrit { exportGenerated = true }\n")
        val enabled = runWithFakeAndroid(root, "kritExportModel", "--dry-run")
        assertTrue(enabled.output.contains(":generateDebugRFile SKIPPED"), enabled.output)
        assertTrue(enabled.output.contains(":processDebugResources SKIPPED"), enabled.output)
        assertTrue(enabled.output.contains(":kspDebugKotlin SKIPPED"), enabled.output)
    }
}
