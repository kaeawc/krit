package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.BuildResult
import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.ValueSource
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * Every task the plugin registers must store and reuse the configuration cache
 * with Isolated Projects on, in a multi-project build, with zero problems: a
 * reported problem fails these builds.
 */
class KritIsolatedProjectsTest {
    @TempDir lateinit var directory: File

    private fun fixture(extraKrit: String = ""): File {
        val root = File(directory, "isolated").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"isolated\"\ninclude(\"a\", \"b\")\n")
        val binary = File(root, "fake-krit")
        binary.writeText(
            """#!/bin/sh
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ] || [ "${'$'}previous" = "--baseline" ]; then printf '{}\n' > "${'$'}arg"; fi
              previous="${'$'}arg"
            done
            """.trimIndent() + "\n"
        )
        binary.setExecutable(true)
        val krit = """
            krit {
                advanced { binary.set(file("${'$'}{rootDir}/fake-krit")) }
                fir = false
                $extraKrit
            }
        """.trimIndent()
        File(root, "build.gradle.kts").writeText(
            "plugins { id(\"dev.jasonpearson.krit.aggregate\") }\n$krit\n" +
                "dependencies { kritAggregate(project(\":a\")); kritAggregate(project(\":b\")) }\n"
        )
        listOf("a", "b").forEach { name ->
            File(root, name).mkdirs()
            File(root, "$name/build.gradle.kts").writeText(
                "plugins { `java-library`; id(\"dev.jasonpearson.krit\") }\n$krit\n"
            )
            File(root, "$name/src/main/kotlin/${name.uppercase()}.kt").apply {
                parentFile.mkdirs()
                writeText("class ${name.uppercase()}\n")
            }
        }
        return root
    }

    private fun run(root: File, vararg args: String): BuildResult = GradleRunner.create()
        .withProjectDir(root)
        .withPluginClasspath()
        .withArguments(*args, "--configuration-cache", "-Dorg.gradle.isolated-projects=true",
            "--offline", "--stacktrace")
        .build()

    private fun assertStoredThenReused(root: File, vararg tasks: String) {
        val first = run(root, *tasks, "--rerun-tasks")
        assertTrue(first.output.contains("Isolated Projects is an incubating feature."), first.output)
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        assertFalse(first.output.contains("problem", ignoreCase = true), first.output)
        tasks.forEach { assertEquals(TaskOutcome.SUCCESS, first.task(it)?.outcome, first.output) }

        val second = run(root, *tasks, "--rerun-tasks")
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        assertFalse(second.output.contains("problem", ignoreCase = true), second.output)
        tasks.forEach { assertEquals(TaskOutcome.SUCCESS, second.task(it)?.outcome, second.output) }
    }

    @ParameterizedTest
    @ValueSource(strings = ["kritCheck", "kritBaseline", "kritFormat"])
    fun `subproject task is isolated projects safe`(task: String) {
        assertStoredThenReused(fixture(), ":a:$task", ":b:$task")
    }

    @Test
    fun `root kritCheckAll is isolated projects safe`() {
        assertStoredThenReused(fixture(), ":kritCheckAll")
    }

    // With every report disabled kritCheck falls back to a SARIF report; that
    // location is wired at configuration time, never read from the project when
    // the task runs.
    @Test
    fun `kritCheck report fallback is isolated projects safe`() {
        val root = fixture(extraKrit = "reports { sarif.required = false }")
        assertStoredThenReused(root, ":a:kritCheck")
        assertTrue(File(root, "a/build/reports/krit/krit.sarif").isFile)
    }
}
