package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.GradleRunner
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * The argv `kritBaseline` and `kritCheck` hand krit. `--create-baseline` takes the
 * baseline path as its value (#1639), and both tasks pass the same `--base-path`,
 * so baseline IDs written by one match the other.
 */
class KritBaselineArgsTest {
    @TempDir lateinit var directory: File

    private fun fixture(): File {
        val root = File(directory, "args").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"args\"\n")
        val binary = File(root, "fake-krit")
        // One argument per line, one blank line between invocations.
        binary.writeText(
            """#!/bin/sh
            for arg in "${'$'}@"; do printf '%s\n' "${'$'}arg" >> "${root.absolutePath}/argv.log"; done
            printf '\n' >> "${root.absolutePath}/argv.log"
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ] || [ "${'$'}previous" = "--create-baseline" ]; then printf '{}\n' > "${'$'}arg"; fi
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
            }
            """.trimIndent() + "\n"
        )
        listOf("main", "test").forEach { set ->
            File(root, "src/$set/kotlin/${set.uppercase()}.kt").apply {
                parentFile.mkdirs()
                writeText("class ${set.uppercase()}\n")
            }
        }
        return root
    }

    private fun invocations(root: File): List<List<String>> =
        File(root, "argv.log").readText().trim().split("\n\n").map { it.lines() }

    private fun valueOf(argv: List<String>, flag: String): String? =
        argv.indexOf(flag).takeIf { it >= 0 }?.let { argv.getOrNull(it + 1) }

    @Test
    fun `kritBaseline passes the baseline path as the create-baseline value`() {
        val root = fixture()
        GradleRunner.create().withProjectDir(root).withPluginClasspath()
            .withArguments("kritBaseline", "--offline").build()

        val argv = invocations(root).single()
        val baseline = File(root, "build/reports/krit/baseline.xml")
        assertEquals(baseline.absolutePath, valueOf(argv, "--create-baseline"), argv.joinToString(" "))
        assertFalse("--baseline" in argv, argv.joinToString(" "))
        assertEquals(root.absolutePath, valueOf(argv, "--base-path"), argv.joinToString(" "))
    }

    @Test
    fun `kritCheck and kritBaseline share the project directory as base path`() {
        val root = fixture()
        GradleRunner.create().withProjectDir(root).withPluginClasspath()
            .withArguments("kritBaseline", "kritCheck", "--offline").build()

        val basePaths = invocations(root).map { valueOf(it, "--base-path") }
        assertEquals(listOf(root.absolutePath, root.absolutePath), basePaths)
    }

    // The default source roots include src/test/kotlin, which need not exist; krit
    // exits 2 on a missing scan path. kritBaseline also honors fir = false, as
    // kritCheck does, so the baseline holds the findings kritCheck reports.
    @Test
    fun `kritBaseline skips missing source roots and passes no-fir`() {
        val root = fixture()
        File(root, "src/test").deleteRecursively()
        GradleRunner.create().withProjectDir(root).withPluginClasspath()
            .withArguments("kritBaseline", "--offline").build()

        val argv = invocations(root).single()
        assertFalse(argv.any { it.endsWith("src/test/kotlin") }, argv.joinToString(" "))
        assertEquals(File(root, "src/main/kotlin").absolutePath, argv.last(), argv.joinToString(" "))
        assertTrue("--no-fir" in argv, argv.joinToString(" "))
    }
}
