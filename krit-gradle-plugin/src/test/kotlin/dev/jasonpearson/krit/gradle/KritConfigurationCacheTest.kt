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

    // Without an explicit binary the plugin resolves (and downloads) krit through a
    // build service. That must wait until a task runs: resolving while the cache
    // entry is stored makes the cached binary's existence a configuration input, so
    // the build after a first download could not reuse the cache.
    @Test
    fun `binary resolution is deferred to task execution`() {
        val root = fixture()
        val build = File(root, "build.gradle.kts")
        build.writeText(
            build.readText().replace(
                "advanced { binary.set(file(\"fake-krit\")) }",
                "advanced { toolVersion = \"0.0.0-unreleased.1\" }",
            )
        )
        val first = run(root, "kritCheck", "kritFormat", "kritBaseline", "--dry-run")
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        val second = run(root, "kritCheck", "kritFormat", "kritBaseline", "--dry-run")
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
    }

    // An explicit binary is tracked by content; the tool version it replaces is not
    // an input, so bumping the plugin's default version does not rerun the task.
    @Test
    fun `explicit binary is tracked by content and tool version is ignored`() {
        val root = fixture()
        run(root, "kritCheck")
        assertEquals(TaskOutcome.UP_TO_DATE, run(root, "kritCheck").task(":kritCheck")?.outcome)

        val build = File(root, "build.gradle.kts")
        build.writeText(build.readText().replace("fir = false", "fir = false\n    advanced { toolVersion = \"9.9.9\" }"))
        assertEquals(TaskOutcome.UP_TO_DATE, run(root, "kritCheck").task(":kritCheck")?.outcome)

        File(root, "fake-krit").appendText("# changed\n")
        assertEquals(TaskOutcome.SUCCESS, run(root, "kritCheck").task(":kritCheck")?.outcome)
    }

    // The binary resolver is a shared build service the tasks declare with
    // usesService; it must not cap how many krit tasks run at once.
    @Test
    fun `krit tasks in different projects run in parallel`() {
        val root = File(directory, "parallel").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"parallel\"\ninclude(\"a\", \"b\")\n")
        val binary = File(root, "fake-krit")
        // Each invocation marks itself started, waits for the other one, and removes
        // its marker on exit, so an overlap is recorded only if both run at once.
        binary.writeText(
            """#!/bin/sh
            dir="${root.absolutePath}"
            touch "${'$'}dir/started-${'$'}${'$'}"
            i=0
            while [ ${'$'}i -lt 150 ]; do
              if [ ${'$'}(ls "${'$'}dir" | grep -c '^started-') -ge 2 ]; then touch "${'$'}dir/overlap"; break; fi
              sleep 0.1
              i=${'$'}((i + 1))
            done
            rm -f "${'$'}dir/started-${'$'}${'$'}"
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ]; then printf '{}\n' > "${'$'}arg"; fi
              previous="${'$'}arg"
            done
            """.trimIndent() + "\n"
        )
        binary.setExecutable(true)
        listOf("a", "b").forEach { name ->
            val dir = File(root, name)
            File(dir, "src/main/kotlin").mkdirs()
            File(dir, "src/main/kotlin/Example.kt").writeText("class Example\n")
            File(dir, "build.gradle.kts").writeText(
                """
                plugins { `java-library`; id("dev.jasonpearson.krit") }
                krit {
                    advanced { binary.set(rootProject.file("fake-krit")) }
                    fir = false
                    exportModel = false
                }
                """.trimIndent() + "\n"
            )
        }
        val result = run(root, ":a:kritCheck", ":b:kritCheck", "--parallel", "--max-workers=4")
        assertEquals(TaskOutcome.SUCCESS, result.task(":a:kritCheck")?.outcome, result.output)
        assertEquals(TaskOutcome.SUCCESS, result.task(":b:kritCheck")?.outcome, result.output)
        assertTrue(File(root, "overlap").isFile, "kritCheck tasks ran one at a time:\n${result.output}")
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
