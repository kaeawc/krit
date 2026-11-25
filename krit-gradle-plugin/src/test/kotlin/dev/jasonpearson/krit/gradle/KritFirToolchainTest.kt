package dev.jasonpearson.krit.gradle

import org.gradle.testkit.runner.BuildResult
import org.gradle.testkit.runner.GradleRunner
import org.gradle.testkit.runner.TaskOutcome
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

/**
 * krit runs FIR on a Java toolchain (passed as JAVA_HOME) and with the krit-fir jar
 * from the `kritFir` configuration (passed as KRIT_FIR_JAR), not on whatever the
 * Gradle daemon's environment holds.
 */
class KritFirToolchainTest {
    @TempDir lateinit var directory: File

    private val marker = "/inherited-from-daemon"

    /** A file Maven repository with krit-fir 9.9.9. */
    private val repository by lazy {
        File(directory, "repo").also { repo ->
            val dir = File(repo, "dev/jasonpearson/krit/krit-fir/9.9.9").apply { mkdirs() }
            File(dir, "krit-fir-9.9.9.jar").writeText("krit-fir 9.9.9\n")
            File(dir, "krit-fir-9.9.9.pom").writeText(
                """<project><modelVersion>4.0.0</modelVersion><groupId>dev.jasonpearson.krit</groupId>
                <artifactId>krit-fir</artifactId><version>9.9.9</version></project>""".trimIndent()
            )
        }
    }

    private fun fixture(build: String): File {
        val root = File(directory, "project").also { it.mkdirs() }
        File(root, "settings.gradle.kts").writeText("rootProject.name = \"toolchain\"\n")
        val binary = File(root, "fake-krit")
        binary.writeText(
            """#!/bin/sh
            printf 'JAVA_HOME=%s\nKRIT_FIR_JAR=%s\n' "${'$'}JAVA_HOME" "${'$'}KRIT_FIR_JAR" > "${root.absolutePath}/env.log"
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
            repositories { maven { url = uri("${repository.toURI()}") } }
            krit {
                exportModel = false
                $build
            }
            """.trimIndent() + "\n"
        )
        File(root, "src/main/kotlin/A.kt").apply { parentFile.mkdirs(); writeText("class A\n") }
        return root
    }

    private fun run(root: File, vararg args: String, env: Map<String, String> = emptyMap()): BuildResult =
        GradleRunner.create()
            .withProjectDir(root)
            .withPluginClasspath()
            .withEnvironment(System.getenv() - "KRIT_FIR_JAR" + mapOf("JAVA_HOME" to marker) + env)
            .withArguments(*args, "--offline", "--stacktrace")
            .build()

    private fun env(root: File): Map<String, String> =
        File(root, "env.log").readLines().associate { it.substringBefore('=') to it.substringAfter('=') }

    private fun javaMajor(home: String): Int {
        val release = File(home, "release").readLines().first { it.startsWith("JAVA_VERSION=") }
        return release.substringAfter('"').substringBefore('"').substringBefore('.').toInt()
    }

    private val explicitBinary = "advanced { binary.set(file(\"fake-krit\")) }"

    @Test
    fun `FIR runs on a Java 21 or newer toolchain`() {
        val root = fixture(explicitBinary)
        run(root, "kritCheck")
        val home = env(root).getValue("JAVA_HOME")
        assertTrue(home != marker, "JAVA_HOME should come from the toolchain, not the daemon")
        val major = javaMajor(home)
        assertTrue(major >= 21 && major >= Runtime.version().feature(), "toolchain Java $major")
    }

    @Test
    fun `an explicit launcher is passed as JAVA_HOME`() {
        val current = Runtime.version().feature()
        val root = fixture(
            "$explicitBinary\n    fir { javaLauncher = javaToolchains.launcherFor { " +
                "languageVersion = JavaLanguageVersion.of($current) } }"
        )
        run(root, "kritCheck")
        assertEquals(current, javaMajor(env(root).getValue("JAVA_HOME")))
    }

    @Test
    fun `with FIR off the environment is left alone`() {
        val root = fixture("$explicitBinary\n    fir = false")
        run(root, "kritCheck", "kritBaseline")
        assertEquals(marker, env(root).getValue("JAVA_HOME"))
        assertEquals("", env(root).getValue("KRIT_FIR_JAR"))
    }

    // The launcher and jar follow the task's own fir switch, not the extension's.
    @Test
    fun `a task that turns FIR on gets the toolchain`() {
        val root = fixture("$explicitBinary\n    fir = false")
        File(root, "build.gradle.kts").appendText(
            "tasks.named<dev.jasonpearson.krit.gradle.KritCheckTask>(\"kritCheck\") { fir = true }\n"
        )
        run(root, "kritCheck")
        assertTrue(env(root).getValue("JAVA_HOME") != marker, "the task runs FIR, so it gets the toolchain JDK")
    }

    @Test
    fun `a pinned kritFir dependency is passed as KRIT_FIR_JAR and tracked by content`() {
        val root = fixture(explicitBinary)
        File(root, "build.gradle.kts").appendText(
            "dependencies { kritFir(\"dev.jasonpearson.krit:krit-fir:9.9.9\") }\n"
        )
        run(root, "kritCheck")
        val jar = File(env(root).getValue("KRIT_FIR_JAR"))
        assertEquals("krit-fir-9.9.9.jar", jar.name)
        assertEquals("krit-fir 9.9.9\n", jar.readText())

        assertEquals(TaskOutcome.UP_TO_DATE, run(root, "kritCheck").task(":kritCheck")?.outcome)
        File(repository, "dev/jasonpearson/krit/krit-fir/9.9.9/krit-fir-9.9.9.jar").writeText("krit-fir 9.9.9 rebuilt\n")
        assertEquals(TaskOutcome.SUCCESS, run(root, "kritCheck", "--refresh-dependencies").task(":kritCheck")?.outcome)
    }

    @Test
    fun `KRIT_FIR_JAR overrides the kritFir configuration`() {
        val root = fixture(explicitBinary)
        File(root, "build.gradle.kts").appendText(
            "dependencies { kritFir(\"dev.jasonpearson.krit:krit-fir:9.9.9\") }\n"
        )
        val local = File(directory, "local/krit-fir.jar").apply { parentFile.mkdirs(); writeText("dev jar\n") }
        run(root, "kritCheck", env = mapOf("KRIT_FIR_JAR" to local.absolutePath))
        assertEquals(local.absolutePath, env(root).getValue("KRIT_FIR_JAR"))
    }

    /** Prints the krit-fir jar kritCheck would run, without running krit. */
    private fun probeFirJar(root: File, vararg args: String, env: Map<String, String> = emptyMap()): String {
        File(root, "build.gradle.kts").appendText(
            """
            tasks.register("printFirJar") {
                val jar = tasks.named<dev.jasonpearson.krit.gradle.KritCheckTask>("kritCheck").map { it.firJar }
                doLast { println("firJar=" + jar.get().files.joinToString { it.name }) }
            }
            """.trimIndent() + "\n"
        )
        val output = run(root, "printFirJar", "--quiet", *args, env = env).output
        return output.lines().single { it.startsWith("firJar=") }.removePrefix("firJar=")
    }

    // Without an explicit binary, the jar defaults to the krit-fir release matching the
    // krit version, resolved from the project's repositories.
    @Test
    fun `the krit-fir jar defaults to the release for the krit version`() {
        assertEquals("krit-fir-9.9.9.jar", probeFirJar(fixture("advanced { toolVersion = \"9.9.9\" }")))
    }

    @Test
    fun `a v-prefixed toolVersion resolves the same krit-fir release`() {
        assertEquals("krit-fir-9.9.9.jar", probeFirJar(fixture("advanced { toolVersion = \"v9.9.9\" }")))
    }

    // A project without a repository that has krit-fir keeps krit's own download,
    // instead of failing the build as an unresolvable default would.
    @Test
    fun `without a repository the default falls back to krit's own download`() {
        val root = fixture("advanced { toolVersion = \"9.9.9\" }")
        File(root, "build.gradle.kts").writeText(
            File(root, "build.gradle.kts").readText().replace(Regex("repositories \\{.*\\}\\n"), "")
        )
        assertEquals("", probeFirJar(root))
    }

    // KRIT_JAR_REPOSITORY is krit's own mirror for the jar; the plugin leaves it in charge.
    @Test
    fun `KRIT_JAR_REPOSITORY keeps krit's own download`() {
        val root = fixture("advanced { toolVersion = \"9.9.9\" }")
        assertEquals("", probeFirJar(root, env = mapOf("KRIT_JAR_REPOSITORY" to "https://mirror.example.invalid/maven")))
    }

    @Test
    fun `a declared kritFir dependency that cannot resolve fails the build`() {
        val root = fixture(explicitBinary)
        File(root, "build.gradle.kts").appendText("dependencies { kritFir(\"dev.jasonpearson.krit:krit-fir:0.0.1\") }\n")
        val result = GradleRunner.create().withProjectDir(root).withPluginClasspath()
            .withEnvironment(System.getenv() - "KRIT_FIR_JAR")
            .withArguments("kritCheck", "--offline").buildAndFail()
        assertTrue(result.output.contains("krit-fir:0.0.1"), result.output)
    }

    @Test
    fun `two kritFir jars fail with a clear message`() {
        val root = fixture(explicitBinary)
        File(root, "build.gradle.kts").appendText(
            "dependencies { kritFir(\"dev.jasonpearson.krit:krit-fir:9.9.9\"); kritFir(\"dev.jasonpearson.krit:krit-fir-extra:9.9.8\") }\n"
        )
        File(repository, "dev/jasonpearson/krit/krit-fir-extra/9.9.8").apply { mkdirs() }.let { dir ->
            File(dir, "krit-fir-extra-9.9.8.jar").writeText("extra\n")
            File(dir, "krit-fir-extra-9.9.8.pom").writeText(
                """<project><modelVersion>4.0.0</modelVersion><groupId>dev.jasonpearson.krit</groupId>
                <artifactId>krit-fir-extra</artifactId><version>9.9.8</version></project>""".trimIndent()
            )
        }
        val result = GradleRunner.create().withProjectDir(root).withPluginClasspath()
            .withEnvironment(System.getenv() - "KRIT_FIR_JAR")
            .withArguments("kritCheck", "--offline").buildAndFail()
        assertTrue(result.output.contains("the kritFir configuration resolved to 2 files"), result.output)
    }

    @Test
    fun `FIR settings store and reuse the configuration cache under isolated projects`() {
        val root = fixture(explicitBinary)
        File(root, "build.gradle.kts").appendText(
            "dependencies { kritFir(\"dev.jasonpearson.krit:krit-fir:9.9.9\") }\n"
        )
        val flags = arrayOf("--configuration-cache", "-Dorg.gradle.isolated-projects=true")
        val first = run(root, "kritCheck", "kritBaseline", *flags)
        assertTrue(first.output.contains("Configuration cache entry stored."), first.output)
        val second = run(root, "kritCheck", "kritBaseline", "--rerun-tasks", *flags)
        assertTrue(second.output.contains("Reusing configuration cache."), second.output)
        assertEquals("krit-fir-9.9.9.jar", File(env(root).getValue("KRIT_FIR_JAR")).name)
    }
}
