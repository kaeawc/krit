package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.BuildResult
import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * kritCheck and kritCheckAll cache keys are relocatable (the same build checked out
 * at two paths shares entries) and correct (a changed dependency jar or krit-fir jar
 * is a miss, even at the same path).
 */
class KritBuildCacheTest {
    @TempDir lateinit var directory: File

    private val cacheDir by lazy { File(directory, "shared-build-cache") }
    private val firJar by lazy { File(directory, "tools/krit-fir.jar").apply { parentFile.mkdirs(); writeText("fir v1\n") } }
    private val tasks = listOf(":lib:kritCheck", ":kritCheckAll")

    private fun checkout(name: String): File {
        val root = File(directory, "$name/checkout").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText(
            """
            rootProject.name = "relocated"
            include("lib")
            buildCache { local { directory = file("${cacheDir.absolutePath}") } }
            """.trimIndent() + "\n"
        )
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
        val krit = "krit { advanced { binary.set(file(\"${'$'}{rootDir}/fake-krit\")) } }"
        File(root, "build.gradle.kts").writeText(
            "plugins { id(\"dev.jasonpearson.krit.aggregate\") }\n$krit\n" +
                "dependencies { kritAggregate(project(\":lib\")) }\n"
        )
        File(root, "lib").mkdirs()
        File(root, "lib/build.gradle.kts").writeText(
            "plugins { `java-library`; id(\"dev.jasonpearson.krit\") }\n$krit\n" +
                "dependencies { implementation(files(\"libs/dep.jar\")) }\n"
        )
        File(root, "lib/libs/dep.jar").apply { parentFile.mkdirs(); writeText("dependency v1\n") }
        File(root, "lib/src/main/kotlin/Lib.kt").apply { parentFile.mkdirs(); writeText("class Lib\n") }
        return root
    }

    private fun run(root: File): BuildResult = GradleRunner.create()
        .withProjectDir(root)
        .withPluginClasspath()
        .withEnvironment(System.getenv() + mapOf("KRIT_FIR_JAR" to firJar.absolutePath))
        .withArguments(*tasks.toTypedArray(), "--build-cache", "--offline", "--stacktrace")
        .build()

    private fun assertOutcomes(expected: TaskOutcome, result: BuildResult) =
        tasks.forEach { assertEquals(expected, result.task(it)?.outcome, "$it\n${result.output}") }

    @Test
    fun `a checkout at another path reuses the cache entries`() {
        assertOutcomes(TaskOutcome.SUCCESS, run(checkout("first")))
        val second = checkout("second")
        assertOutcomes(TaskOutcome.FROM_CACHE, run(second))
        assertEquals("{}\n", File(second, "lib/build/reports/krit/krit.sarif").readText())
    }

    // The model lists the jar by path, so only content-hashing the jar sees a
    // re-published SNAPSHOT at the same path.
    @Test
    fun `a changed dependency jar at the same path reruns the check`() {
        val root = checkout("first")
        run(root)
        File(root, "lib/libs/dep.jar").writeText("dependency v2, re-published\n")
        assertOutcomes(TaskOutcome.SUCCESS, run(root))
    }

    @Test
    fun `a changed krit-fir jar reruns the check`() {
        val root = checkout("first")
        run(root)
        firJar.writeText("fir v2\n")
        assertOutcomes(TaskOutcome.SUCCESS, run(root))
    }

    @Test
    fun `an unchanged build is up to date`() {
        val root = checkout("first")
        run(root)
        assertOutcomes(TaskOutcome.UP_TO_DATE, run(root))
    }

    @Test
    fun `the krit-fir jar is not an input when FIR is off`() {
        val first = checkout("first")
        listOf("build.gradle.kts", "lib/build.gradle.kts").forEach {
            File(first, it).appendText("krit { fir = false }\n")
        }
        run(first)
        firJar.writeText("fir v2\n")
        val second = checkout("second")
        listOf("build.gradle.kts", "lib/build.gradle.kts").forEach {
            File(second, it).appendText("krit { fir = false }\n")
        }
        assertOutcomes(TaskOutcome.FROM_CACHE, run(second))
    }

    // The krit-fir jar is an input exactly when the task itself runs FIR, whatever
    // the extension says.
    @Test
    fun `a task that turns FIR on tracks the krit-fir jar`() {
        val root = checkout("first")
        File(root, "lib/build.gradle.kts").appendText(
            "krit { fir = false }\ntasks.named<dev.jasonpearson.krit.gradle.KritCheckTask>(\"kritCheck\") { fir = true }\n"
        )
        run(root)
        firJar.writeText("fir v2\n")
        assertEquals(TaskOutcome.SUCCESS, run(root).task(":lib:kritCheck")?.outcome)
    }

    @Test
    fun `a task that turns FIR off ignores the krit-fir jar`() {
        val root = checkout("first")
        File(root, "lib/build.gradle.kts").appendText(
            "tasks.named<dev.jasonpearson.krit.gradle.KritCheckTask>(\"kritCheck\") { fir = false }\n"
        )
        run(root)
        firJar.writeText("fir v2\n")
        assertEquals(TaskOutcome.UP_TO_DATE, run(root).task(":lib:kritCheck")?.outcome)
    }
}
