package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.BuildResult
import org.gradle.testkit.runner.GradleRunner
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * The krit tasks of a build share warm FIR JVMs through [KritFirDaemonPool]: each
 * krit run gets `KRIT_FIR_DAEMON_POOL=<pool>-<slot>`, and no two concurrent runs
 * share a slot.
 */
class KritFirDaemonPoolTest {
    @TempDir lateinit var directory: File

    private val projects = listOf("a", "b", "c", "d")

    private fun fixture(firBlock: String = ""): File {
        val root = File(directory, "pool").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText(
            "rootProject.name = \"pool\"\ninclude(${projects.joinToString { "\"$it\"" }})\n"
        )
        val binary = File(root, "fake-krit")
        // One line per run: "<start ns> <end ns> <pool slot> <heap>"; a short sleep makes
        // parallel runs overlap.
        binary.writeText(
            """#!/bin/sh
            start=${'$'}(date +%s%N)
            sleep 0.5
            printf '%s %s %s %s\n' "${'$'}start" "${'$'}(date +%s%N)" "${'$'}{KRIT_FIR_DAEMON_POOL:--}" "${'$'}{KRIT_FIR_XMX:--}" >> "${root.absolutePath}/runs.log"
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ]; then printf '{}\n' > "${'$'}arg"; fi
              previous="${'$'}arg"
            done
            """.trimIndent() + "\n"
        )
        binary.setExecutable(true)
        projects.forEach { name ->
            File(root, name).mkdirs()
            File(root, "$name/build.gradle.kts").writeText(
                """
                plugins { `java-library`; id("dev.jasonpearson.krit") }
                krit {
                    advanced { binary.set(file("${'$'}{rootDir}/fake-krit")) }
                    exportModel = false
                    $firBlock
                }
                """.trimIndent() + "\n"
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
        .withEnvironment(System.getenv() - "KRIT_FIR_JAR")
        .withArguments(*args, "--offline", "--stacktrace")
        .build()

    private data class Run(val start: Long, val end: Long, val slot: String, val heap: String)

    private fun runs(root: File): List<Run> = File(root, "runs.log").readLines().map { line ->
        val (start, end, slot, heap) = line.split(" ")
        Run(start.toLong(), end.toLong(), slot, heap)
    }

    private fun kritChecks() = projects.map { ":$it:kritCheck" }.toTypedArray()

    @Test
    fun `sequential checks reuse one pool slot`() {
        val root = fixture()
        run(root, *kritChecks())
        val slots = runs(root).map { it.slot }.toSet()
        assertEquals(1, slots.size, slots.toString())
        assertTrue(slots.single().matches(Regex("gradle-[0-9a-f]{12}-0")), slots.toString())
    }

    @Test
    fun `parallel checks never share a slot at the same time`() {
        val root = fixture("fir { maxDaemons = 2 }")
        run(root, *kritChecks(), "--parallel", "--max-workers=4")
        val all = runs(root)
        assertEquals(4, all.size)
        assertTrue(all.map { it.slot.substringAfterLast('-') }.toSet().all { it in setOf("0", "1") }, all.toString())
        for (i in all.indices) for (j in i + 1 until all.size) {
            val overlap = all[i].start < all[j].end && all[j].start < all[i].end
            if (overlap) assertTrue(all[i].slot != all[j].slot, "overlapping runs share ${all[i].slot}")
        }
        assertTrue(all.indices.any { i -> all.indices.any { j -> i != j && all[i].start < all[j].end && all[j].start < all[i].end } },
            "two slots should let two checks run at once: $all")
    }

    @Test
    fun `one daemon serializes the FIR checks`() {
        val root = fixture("fir { maxDaemons = 1 }")
        run(root, *kritChecks(), "--parallel", "--max-workers=4")
        val all = runs(root).sortedBy { it.start }
        assertEquals(1, all.map { it.slot }.toSet().size)
        all.zipWithNext().forEach { (first, second) -> assertTrue(first.end <= second.start, "runs overlap: $all") }
    }

    @Test
    fun `maxHeap is passed as KRIT_FIR_XMX`() {
        val root = fixture("fir { maxHeap = \"3g\" }")
        run(root, ":a:kritCheck")
        assertEquals("3g", runs(root).single().heap)
    }

    @Test
    fun `an invalid maxHeap fails with the expected format`() {
        val root = fixture("fir { maxHeap = \"lots\" }")
        val result = GradleRunner.create().withProjectDir(root).withPluginClasspath()
            .withArguments(":a:kritCheck", "--offline").buildAndFail()
        assertTrue(result.output.contains("krit fir.maxHeap must look like 4g or 2048m, not 'lots'"), result.output)
    }

    @Test
    fun `with FIR off there is no pool`() {
        val root = fixture("fir = false")
        run(root, *kritChecks(), "--parallel")
        assertEquals(setOf("-"), runs(root).map { it.slot }.toSet())
    }

    @Test
    fun `the pool stores and reuses the configuration cache under isolated projects`() {
        val root = fixture()
        val flags = arrayOf("--configuration-cache", "-Dorg.gradle.isolated-projects=true")
        val first = run(root, *kritChecks(), *flags)
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        val second = run(root, *kritChecks(), "--rerun-tasks", *flags)
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        // The configuration cache runs tasks in parallel, so both default slots may serve.
        val slots = runs(root).map { it.slot }.toSet()
        assertEquals(1, slots.map { it.substringBeforeLast('-') }.toSet().size, slots.toString())
        assertTrue(slots.all { it.substringAfterLast('-') in setOf("0", "1") }, slots.toString())
    }
}
