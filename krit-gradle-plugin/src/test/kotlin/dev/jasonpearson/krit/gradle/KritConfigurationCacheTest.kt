package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.ValueSource
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * The plugin declares configuration-cache support on the Plugin Portal, so every
 * task it registers must store and reuse the configuration cache without problems.
 */
class KritConfigurationCacheTest {
    @TempDir lateinit var directory: File

    private fun fixture(extraKrit: String = ""): File {
        val root = File(directory, "fixture").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"fixture\"\n")
        val binary = File(root, "fake-krit")
        binary.writeText(
            """#!/bin/sh
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ]; then printf '{}\n' > "${'$'}arg"; fi
              previous="${'$'}arg"
            done
            """.trimIndent() + "\n"
        )
        binary.setExecutable(true)
        File(root, "build.gradle.kts").writeText(
            """
            plugins { `java-library`; id("dev.jasonpearson.krit") }
            krit {
                advanced { binary.set(file("fake-krit")) }
                fir = false
                exportModel = false
                $extraKrit
            }
            """.trimIndent() + "\n"
        )
        val source = File(root, "src/main/kotlin/Example.kt")
        source.parentFile.mkdirs()
        source.writeText("class Example\n")
        return root
    }

    private fun run(root: File, vararg args: String) = GradleRunner.create()
        .withProjectDir(root)
        .withPluginClasspath()
        .withArguments(*args, "--configuration-cache", "--offline", "--stacktrace")
        .build()

    private fun assertStoredThenReused(root: File, task: String) {
        val first = run(root, task, "--rerun-tasks")
        assertEquals(TaskOutcome.SUCCESS, first.task(":$task")?.outcome, first.output)
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        val second = run(root, task, "--rerun-tasks")
        assertEquals(TaskOutcome.SUCCESS, second.task(":$task")?.outcome, second.output)
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
    }

    @ParameterizedTest
    @ValueSource(strings = ["kritCheck", "kritFormat", "kritBaseline"])
    fun `task stores and reuses the configuration cache`(task: String) {
        assertStoredThenReused(fixture(), task)
    }

    // With every report disabled, kritCheck falls back to a SARIF file under the
    // build directory, resolved while the task runs.
    @Test
    fun `kritCheck fallback report path is configuration-cache safe`() {
        val root = fixture("reports { sarif.required = false }")
        assertStoredThenReused(root, "kritCheck")
        assertTrue(File(root, "build/reports/krit/krit.sarif").isFile)
    }
}
