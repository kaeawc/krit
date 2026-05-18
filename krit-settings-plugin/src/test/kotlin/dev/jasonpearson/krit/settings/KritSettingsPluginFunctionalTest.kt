package dev.jasonpearson.krit.settings

import org.gradle.testkit.runner.GradleRunner
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * End-to-end coverage: a real multi-module Gradle build that applies
 * `dev.jasonpearson.krit.settings` from `settings.gradle.kts` and proves the
 * settings plugin auto-applies `dev.jasonpearson.krit` to each Kotlin/Java
 * subproject (and skips listed paths).
 *
 * Requires `krit.gradle.plugin.dir` to point at a directory whose
 * `build.gradle.kts` exposes `dev.jasonpearson.krit`. The functional test's
 * generated consumer `settings.gradle.kts` does `includeBuild(...)` on that
 * directory so the project plugin resolves end-to-end without staging to
 * mavenLocal first. Set via the `systemProperty` block in this module's
 * `build.gradle.kts`.
 */
class KritSettingsPluginFunctionalTest {

    @TempDir
    lateinit var projectDir: File

    @Test
    fun `auto-applies krit project plugin to kotlin subprojects`() {
        val kritGradlePluginDir = locateKritGradlePlugin() ?: return
        writeMultiProjectBuild(kritGradlePluginDir, skipList = "")

        val result = GradleRunner.create()
            .withProjectDir(projectDir)
            .withPluginClasspath()
            .withArguments(":lib1:tasks", "--all", "--stacktrace")
            .forwardOutput()
            .build()

        assertTrue("kritCheck" in result.output,
            "Expected :lib1:kritCheck to be registered by the settings plugin")
    }

    @Test
    fun `skip list excludes the named subproject from auto-application`() {
        val kritGradlePluginDir = locateKritGradlePlugin() ?: return
        writeMultiProjectBuild(kritGradlePluginDir, skipList = "\":skipped\"")

        // lib1 still gets kritCheck...
        val included = GradleRunner.create()
            .withProjectDir(projectDir)
            .withPluginClasspath()
            .withArguments(":lib1:tasks", "--all", "--stacktrace")
            .forwardOutput()
            .build()
        assertTrue("kritCheck" in included.output,
            ":lib1 should still have kritCheck registered")

        // ...but the skipped project does not.
        val skipped = GradleRunner.create()
            .withProjectDir(projectDir)
            .withPluginClasspath()
            .withArguments(":skipped:tasks", "--all", "--stacktrace")
            .forwardOutput()
            .build()
        assertFalse("kritCheck" in skipped.output,
            ":skipped should NOT have kritCheck registered; output was:\n${skipped.output}")
    }

    @Test
    fun `inherited config flows into per-project conventions`() {
        val kritGradlePluginDir = locateKritGradlePlugin() ?: return
        writeMultiProjectBuild(kritGradlePluginDir, skipList = "")

        // Probe task we register in lib1's build file (below) prints the
        // resolved KritExtension.ignoreFailures convention so we can assert
        // the inherited value reached the subproject.
        val result = GradleRunner.create()
            .withProjectDir(projectDir)
            .withPluginClasspath()
            .withArguments(":lib1:reportKritIgnoreFailures", "--quiet", "--stacktrace")
            .forwardOutput()
            .build()

        assertTrue("ignoreFailures=true" in result.output,
            "expected inherited ignoreFailures=true on :lib1; got:\n${result.output}")
    }

    private fun writeMultiProjectBuild(kritGradlePluginDir: String, skipList: String) {
        // Escape backslashes so Windows paths survive the heredoc.
        val safePath = kritGradlePluginDir.replace("\\", "\\\\")
        val skipLine = if (skipList.isBlank()) "" else "    skip($skipList)"
        File(projectDir, "settings.gradle.kts").writeText(
            """
            pluginManagement {
                repositories {
                    mavenCentral()
                    gradlePluginPortal()
                }
                includeBuild("$safePath")
            }
            plugins {
                id("dev.jasonpearson.krit.settings")
            }
            rootProject.name = "demo"
            include(":lib1", ":lib2", ":skipped")

            krit {
                ignoreFailures = true
            $skipLine
            }
            """.trimIndent()
        )

        File(projectDir, "build.gradle.kts").writeText("")

        // Two Kotlin subprojects; both should pick up auto-application.
        listOf("lib1", "lib2").forEach { name ->
            val dir = File(projectDir, name).apply { mkdirs() }
            File(dir, "build.gradle.kts").writeText(
                """
                plugins {
                    kotlin("jvm") version "2.3.21"
                }

                // Probe task: prints the inherited convention so the
                // functional test can verify the settings → project flow.
                // Looks up the krit extension by name to avoid the
                // classloader mismatch when the build script and plugin
                // classpath are separate (TestKit + includeBuild).
                tasks.register("reportKritIgnoreFailures") {
                    notCompatibleWithConfigurationCache(
                        "test-only probe; reads extension at execution time"
                    )
                    val kritExt = project.extensions.findByName("krit")
                        ?: error("krit extension was not registered on " + project.path)
                    doLast {
                        val prop = kritExt::class.java.getMethod("getIgnoreFailures").invoke(kritExt)
                        val value = prop::class.java.getMethod("get").invoke(prop)
                        println("ignoreFailures=" + value)
                    }
                }
                """.trimIndent()
            )
        }

        // A "skipped" subproject that also applies Kotlin — the settings
        // plugin should refuse to apply krit here when the path is in the
        // skip list.
        val skippedDir = File(projectDir, "skipped").apply { mkdirs() }
        File(skippedDir, "build.gradle.kts").writeText(
            """
            plugins {
                kotlin("jvm") version "2.3.21"
            }
            """.trimIndent()
        )
    }

    private fun locateKritGradlePlugin(): String? {
        val sys = System.getProperty("krit.gradle.plugin.dir")
        if (sys != null && File(sys, "build.gradle.kts").isFile) return sys
        // Fallback for IDE runs: look for a sibling directory.
        val cwd = File(System.getProperty("user.dir"))
        val candidate = cwd.parentFile?.resolve("krit-gradle-plugin")
        return if (candidate?.resolve("build.gradle.kts")?.isFile == true) candidate.absolutePath else null
    }
}
