package dev.jasonpearson.krit.gradle

import groovy.json.JsonSlurper
import org.gradle.testkit.runner.BuildResult
import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * `krit { fir { useBuiltDependencies = true } }`: the leaf of a 3-project chain gets
 * its upstream projects' compiled classes through Gradle (`--gradle-project-outputs`)
 * instead of krit compiling their sources with it.
 */
class KritBuiltDependenciesTest {
    @TempDir lateinit var directory: File

    private fun fixture(enabled: Boolean = true): File {
        val root = File(directory, "chain").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"chain\"\ninclude(\"core\", \"mid\", \"leaf\")\n")
        val binary = File(root, "fake-krit")
        // Records the argv and a copy of the project-outputs file krit was given.
        binary.writeText(
            """#!/bin/sh
            printf '%s\n' "${'$'}*" >> "${root.absolutePath}/argv.log"
            previous=""
            for arg in "${'$'}@"; do
              if [ "${'$'}previous" = "-o" ]; then printf '{}\n' > "${'$'}arg"; fi
              if [ "${'$'}previous" = "--gradle-project-outputs" ]; then cp "${'$'}arg" "${root.absolutePath}/outputs.json"; fi
              previous="${'$'}arg"
            done
            """.trimIndent() + "\n"
        )
        binary.setExecutable(true)
        val krit = """
            krit {
                advanced { binary.set(file("${'$'}{rootDir}/fake-krit")) }
                fir { useBuiltDependencies = $enabled }
            }
        """.trimIndent()
        val deps = mapOf("core" to "", "mid" to "api(project(\":core\"))", "leaf" to "implementation(project(\":mid\"))")
        deps.forEach { (name, dependency) ->
            val dir = File(root, name).also { it.mkdirs() }
            File(dir, "build.gradle.kts").writeText(
                "plugins { `java-library`; id(\"dev.jasonpearson.krit\") }\n$krit\ndependencies { $dependency }\n"
            )
            val cls = name.replaceFirstChar(Char::uppercase)
            File(dir, "src/main/java/$name/$cls.java").apply {
                parentFile.mkdirs()
                writeText("package $name;\npublic class $cls {\n    public int value() { return 1; }\n}\n")
            }
            File(dir, "src/main/kotlin/$cls.kt").apply { parentFile.mkdirs(); writeText("class ${cls}Kt\n") }
        }
        return root
    }

    private fun run(root: File, vararg args: String): BuildResult = GradleRunner.create()
        .withProjectDir(root)
        .withPluginClasspath()
        .withEnvironment(System.getenv() - "KRIT_FIR_JAR")
        .withArguments(*args, "--offline", "--stacktrace")
        .build()

    @Suppress("UNCHECKED_CAST")
    private fun outputs(root: File): Map<String, List<String>> =
        (JsonSlurper().parse(File(root, "outputs.json")) as Map<String, Any>)["projects"] as Map<String, List<String>>

    @Test
    fun `the leaf check gets upstream classes built by Gradle`() {
        val root = fixture()
        val result = run(root, ":leaf:kritCheck")
        assertEquals(TaskOutcome.SUCCESS, result.task(":leaf:kritCheck")?.outcome, result.output)
        assertEquals(TaskOutcome.SUCCESS, result.task(":mid:compileJava")?.outcome, result.output)
        assertEquals(TaskOutcome.SUCCESS, result.task(":core:compileJava")?.outcome, result.output)
        assertNull(result.task(":leaf:compileJava"), "the checked project itself is not compiled")

        val projects = outputs(root)
        assertEquals(setOf(":core", ":mid"), projects.keys, projects.toString())
        assertEquals(listOf(modelPath(File(root, "mid/build/classes/java/main"))), projects[":mid"])
        assertEquals(listOf(modelPath(File(root, "core/build/classes/java/main"))), projects[":core"])
    }

    @Test
    fun `an implementation-only upstream change keeps the check up to date`() {
        val root = fixture()
        run(root, ":leaf:kritCheck")
        File(root, "mid/src/main/java/mid/Mid.java").writeText(
            "package mid;\npublic class Mid {\n    public int value() { return 2; }\n}\n"
        )
        val bodyChange = run(root, ":leaf:kritCheck")
        assertEquals(TaskOutcome.SUCCESS, bodyChange.task(":mid:compileJava")?.outcome)
        assertEquals(TaskOutcome.UP_TO_DATE, bodyChange.task(":leaf:kritCheck")?.outcome, bodyChange.output)

        File(root, "mid/src/main/java/mid/Mid.java").writeText(
            "package mid;\npublic class Mid {\n    public int value() { return 2; }\n    public int added() { return 3; }\n}\n"
        )
        assertEquals(TaskOutcome.SUCCESS, run(root, ":leaf:kritCheck").task(":leaf:kritCheck")?.outcome)
    }

    @Test
    fun `off by default the check neither compiles upstream nor passes outputs`() {
        val root = fixture(enabled = false)
        val result = run(root, ":leaf:kritCheck")
        assertNull(result.task(":mid:compileJava"), result.output)
        assertFalse(File(root, "argv.log").readText().contains("--gradle-project-outputs"))
    }

    @Test
    fun `built dependencies store and reuse the configuration cache under isolated projects`() {
        val root = fixture()
        val flags = arrayOf("--configuration-cache", "-Dorg.gradle.isolated-projects=true")
        val first = run(root, ":leaf:kritCheck", *flags)
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        val second = run(root, ":leaf:kritCheck", "--rerun-tasks", *flags)
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        assertEquals(setOf(":core", ":mid"), outputs(root).keys)
    }
}
