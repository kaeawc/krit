package dev.jasonpearson.krit.custom

import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.util.jar.JarFile

/**
 * End-to-end check that the acceptance criterion holds: a consumer project
 * applying the plugin and writing a single `class MyRule : KritRule` produces
 * a working rule jar with `./gradlew kritRuleJar`.
 *
 * Requires `dev.jasonpearson.krit:krit-rule-api` to be resolvable. The
 * surrounding harness publishes a `0.0.0-test` snapshot to `~/.m2` before
 * running these tests; if that snapshot is missing the test is skipped so
 * we don't fail the suite on a clean workstation.
 */
class KritCustomRulePluginFunctionalTest {

    @TempDir
    lateinit var projectDir: File

    @Test
    fun `single KritRule class produces a stamped rule jar`() {
        val ruleApi = locateLocalRuleApi() ?: return // skip if not staged

        File(projectDir, "settings.gradle.kts").writeText(
            """
            pluginManagement {
                repositories {
                    mavenLocal()
                    gradlePluginPortal()
                    mavenCentral()
                }
            }
            dependencyResolutionManagement {
                repositories {
                    mavenLocal()
                    mavenCentral()
                }
            }
            rootProject.name = "my-rules"
            """.trimIndent()
        )

        File(projectDir, "build.gradle.kts").writeText(
            """
            plugins {
                id("dev.jasonpearson.krit.custom")
            }
            kritCustomRules {
                ruleApiVersion.set("${ruleApi.version}")
                sdkVersion.set("${ruleApi.version}")
                vendorId.set("acme")
            }
            kotlin { jvmToolchain(21) }
            """.trimIndent()
        )

        val source = File(projectDir, "src/main/kotlin/com/example/MyRule.kt")
        source.parentFile.mkdirs()
        source.writeText(
            """
            package com.example

            import dev.jasonpearson.krit.api.Finding
            import dev.jasonpearson.krit.api.KritFile
            import dev.jasonpearson.krit.api.KritRule
            import dev.jasonpearson.krit.api.RuleContext

            class MyRule : KritRule {
                override fun check(file: KritFile, ctx: RuleContext): List<Finding> = emptyList()
            }
            """.trimIndent()
        )

        val result = GradleRunner.create()
            .withProjectDir(projectDir)
            .withPluginClasspath()
            .withArguments("kritRuleJar", "--stacktrace")
            .forwardOutput()
            .build()

        assertEquals(TaskOutcome.SUCCESS, result.task(":kritRuleJar")!!.outcome)

        val jar = File(projectDir, "build/libs").listFiles()?.firstOrNull { it.name.endsWith(".jar") }
        assertTrue(jar != null && jar.isFile, "rule jar should be produced under build/libs")

        JarFile(jar!!).use { jf ->
            val manifest = jf.manifest!!.mainAttributes
            assertEquals(ruleApi.version, manifest.getValue("Krit-SDK-Version"))
            assertEquals("acme", manifest.getValue("Krit-Vendor-Id"))

            val service = jf.getEntry("META-INF/services/dev.jasonpearson.krit.api.KritRule")
            assertTrue(service != null, "service file should be present in jar")
            val body = jf.getInputStream(service).bufferedReader().readText()
            assertTrue(
                body.lineSequence().any { !it.startsWith("#") && it.trim() == "com.example.MyRule" },
                "service file should list MyRule:\n$body"
            )
        }
    }

    private data class LocalRuleApi(val version: String)

    /**
     * Look for any `~/.m2/repository/dev/jasonpearson/krit/krit-rule-api/<v>/`
     * directory whose jar is on disk. Returning null causes the test to skip.
     */
    private fun locateLocalRuleApi(): LocalRuleApi? {
        val home = System.getProperty("user.home")
        val dir = File("$home/.m2/repository/dev/jasonpearson/krit/krit-rule-api")
        if (!dir.isDirectory) return null
        val versionDir = dir.listFiles { f -> f.isDirectory }?.firstOrNull { v ->
            File(v, "krit-rule-api-${v.name}.jar").isFile
        } ?: return null
        return LocalRuleApi(version = versionDir.name)
    }
}
