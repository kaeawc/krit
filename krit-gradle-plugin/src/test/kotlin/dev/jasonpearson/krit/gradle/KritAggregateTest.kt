package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.BuildResult
import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/** `kritCheckAll`: one krit process over the whole build, fed by every project's model. */
class KritAggregateTest {
    @TempDir lateinit var directory: File

    private val libraries = listOf("a", "b", "nested:c")

    /**
     * Root with the aggregate plugin, three krit projects (one nested) and one plain
     * project without krit. The fake binary logs each invocation, one line per run.
     */
    private fun fixture(extraRoot: String = ""): File {
        val root = File(directory, "build-root").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText(
            "rootProject.name = \"build-root\"\ninclude(\"a\", \"b\", \"nested:c\", \"plain\")\n"
        )
        val binary = File(root, "fake-krit")
        binary.writeText(
            """#!/bin/sh
            printf 'shards=%s args=%s\n' "${'$'}{KRIT_MODULE_SHARDS:-}" "${'$'}*" >> "${root.absolutePath}/invocations.log"
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ]; then printf '{}\n' > "${'$'}arg"; fi
              if [ "${'$'}previous" = "--report" ]; then printf '{}\n' > "${'$'}{arg#*=}"; fi
              previous="${'$'}arg"
            done
            """.trimIndent() + "\n"
        )
        binary.setExecutable(true)
        File(root, "build.gradle.kts").writeText(
            """
            plugins { id("dev.jasonpearson.krit.aggregate") }
            krit {
                advanced { binary.set(file("fake-krit")) }
                fir = false
                $extraRoot
            }
            dependencies {
                ${(libraries + "plain").joinToString("\n                ") { "kritAggregate(project(\":$it\"))" }}
            }
            """.trimIndent() + "\n"
        )
        libraries.forEach { path ->
            val dir = File(root, path.replace(':', '/')).also { it.mkdirs() }
            File(dir, "build.gradle.kts").writeText(
                "plugins { `java-library`; id(\"dev.jasonpearson.krit\") }\n"
            )
            File(dir, "src/main/kotlin/${path.substringAfterLast(':').uppercase()}.kt").apply {
                parentFile.mkdirs()
                writeText("class ${path.substringAfterLast(':').uppercase()}\n")
            }
        }
        File(root, "plain").mkdirs()
        File(root, "plain/build.gradle.kts").writeText("")
        return root
    }

    private fun runner(root: File, vararg args: String) = GradleRunner.create()
        .withProjectDir(root)
        .withPluginClasspath()
        .withArguments(*args, "--offline", "--stacktrace")

    private fun run(root: File, vararg args: String): BuildResult = runner(root, *args).build()

    private fun invocations(root: File): List<String> =
        File(root, "invocations.log").takeIf { it.isFile }?.readLines().orEmpty()

    @Test
    fun `kritCheckAll runs krit once over the build root after every project's model export`() {
        val root = fixture()
        val result = run(root, "kritCheckAll")

        assertEquals(TaskOutcome.SUCCESS, result.task(":kritCheckAll")?.outcome, result.output)
        listOf(":a", ":b", ":nested:c").forEach { path ->
            assertEquals(TaskOutcome.SUCCESS, result.task("$path:kritExportModel")?.outcome, result.output)
        }
        assertEquals(null, result.task(":a:kritCheck"), "per-project kritCheck must not run")
        listOf("a", "b", "nested__c").forEach { name ->
            assertTrue(File(root, ".krit/gradle-model/$name.json").isFile, name)
        }

        val calls = invocations(root)
        assertEquals(1, calls.size, calls.joinToString("\n"))
        val call = calls.single()
        assertTrue(call.endsWith(" ${root.canonicalFile.absolutePath}") || call.endsWith(" ${root.absolutePath}"), call)
        assertTrue(call.contains("--format=sarif"), call)
        assertTrue(call.contains("--no-fir"), call)
        assertTrue(call.startsWith("shards= "), "module sharding is opt-in: $call")
        assertTrue(File(root, "build/reports/krit/all/krit.sarif").isFile)
    }

    @Test
    fun `kritCheckAll is up to date until a source or model changes`() {
        val root = fixture()
        run(root, "kritCheckAll")
        assertEquals(TaskOutcome.UP_TO_DATE, run(root, "kritCheckAll").task(":kritCheckAll")?.outcome)

        File(root, "b/src/main/kotlin/B.kt").writeText("class B { val changed = 1 }\n")
        assertEquals(TaskOutcome.SUCCESS, run(root, "kritCheckAll").task(":kritCheckAll")?.outcome)

        // A model change (here a new source set directory) also reruns the scan.
        File(root, "a/build.gradle.kts").appendText(
            "sourceSets { main { java.srcDir(\"src/extra/java\") } }\n"
        )
        assertEquals(TaskOutcome.SUCCESS, run(root, "kritCheckAll").task(":kritCheckAll")?.outcome)
        assertEquals(3, invocations(root).size)
    }

    @Test
    fun `build outputs are not scan inputs`() {
        val root = fixture()
        run(root, "kritCheckAll")
        File(root, "a/build/generated/Stale.kt").apply { parentFile.mkdirs(); writeText("class Stale\n") }
        assertEquals(TaskOutcome.UP_TO_DATE, run(root, "kritCheckAll").task(":kritCheckAll")?.outcome)
    }

    @Test
    fun `moduleShards is passed to krit as KRIT_MODULE_SHARDS`() {
        val root = fixture(extraRoot = "advanced { moduleShards = \"auto\" }")
        run(root, "kritCheckAll")
        assertTrue(invocations(root).single().startsWith("shards=auto "), invocations(root).joinToString())
    }

    @Test
    fun `several report formats come from one krit run`() {
        val root = fixture(extraRoot = "reports { json.required = true }")
        run(root, "kritCheckAll")
        val sarif = File(root, "build/reports/krit/all/krit.sarif")
        val json = File(root, "build/reports/krit/all/krit.json")
        assertTrue(sarif.isFile)
        assertTrue(json.isFile)
        val call = invocations(root).single()
        assertTrue(call.contains("--report sarif=${sarif.absolutePath} --report json=${json.absolutePath}"), call)
        assertFalse(call.contains(" -o "), call)
    }

    // krit 0.5.0 and earlier reject --report FORMAT=PATH ("unknown format", exit 2).
    @Test
    fun `several reports fall back to one run each on a krit without --report`() {
        val root = fixture(extraRoot = "reports { json.required = true }")
        val binary = File(root, "fake-krit")
        binary.writeText(binary.readText().replaceFirst("\n", "\n" +
            "case \"${'$'}*\" in *--report*) echo 'error: unknown format: sarif=x' >&2; exit 2;; esac\n"))
        run(root, "kritCheckAll")
        val calls = invocations(root)
        // The rejected --report run exits before the fake logs it.
        assertEquals(2, calls.size, calls.joinToString("\n"))
        assertTrue(calls[0].contains("--format=sarif") && calls[1].contains("--format=json"), calls.joinToString("\n"))
        assertTrue(File(root, "build/reports/krit/all/krit.sarif").isFile)
        assertTrue(File(root, "build/reports/krit/all/krit.json").isFile)
    }

    @Test
    fun `findings fail the build unless ignoreFailures is set`() {
        val root = fixture()
        File(root, "fake-krit").appendText("exit 1\n")
        val failed = runner(root, "kritCheckAll").buildAndFail()
        assertTrue(failed.output.contains("krit analysis found issues (exit code 1)"), failed.output)

        File(root, "build.gradle.kts").writeText(
            File(root, "build.gradle.kts").readText().replace("fir = false", "fir = false\n    ignoreFailures = true")
        )
        assertEquals(TaskOutcome.SUCCESS, run(root, "kritCheckAll").task(":kritCheckAll")?.outcome)
    }

    @Test
    fun `kritCheckAll stores and reuses the configuration cache under isolated projects`() {
        val root = fixture()
        val flags = arrayOf("--configuration-cache", "-Dorg.gradle.isolated-projects=true")
        val first = run(root, "kritCheckAll", *flags)
        assertTrue(first.output.contains("Isolated Projects is an incubating feature."), first.output)
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        assertFalse(first.output.contains("problems were found"), first.output)
        assertEquals(TaskOutcome.SUCCESS, first.task(":kritCheckAll")?.outcome)

        val second = run(root, "kritCheckAll", "--rerun-tasks", *flags)
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        assertEquals(TaskOutcome.SUCCESS, second.task(":kritCheckAll")?.outcome)
        assertEquals(2, invocations(root).size)
    }

    @Test
    fun `a JVM root project contributes its own model`() {
        val root = fixture()
        File(root, "build.gradle.kts").writeText(
            File(root, "build.gradle.kts").readText()
                .replace("plugins { id(\"dev.jasonpearson.krit.aggregate\") }",
                    "plugins { `java-library`; id(\"dev.jasonpearson.krit.aggregate\") }")
        )
        val result = run(root, "kritCheckAll")
        assertEquals(TaskOutcome.SUCCESS, result.task(":kritExportModel")?.outcome, result.output)
        assertTrue(File(root, ".krit/gradle-model/_root.json").isFile)
    }
}
