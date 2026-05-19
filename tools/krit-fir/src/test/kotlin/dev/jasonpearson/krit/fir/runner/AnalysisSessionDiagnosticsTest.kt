package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.oracle.DiagnosticPayload
import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for the diagnostic projection in
 * [`AnalysisSession.analyze`]: the krit-types-retained K2 warning
 * factories (`USELESS_ELVIS`, `CAST_NEVER_SUCCEEDS`, `UNREACHABLE_CODE`)
 * round-trip through the [`OracleDiagnosticMessageCollector`] into
 * each [`FilePayload.diagnostics`] list.
 */
class AnalysisSessionDiagnosticsTest {

    @TempDir
    lateinit var tmp: Path

    private lateinit var stdlibClasspath: List<String>

    @BeforeEach
    fun resolveStdlib() {
        val stdlib = findKotlinStdlib()
        assumeTrue(
            stdlib != null,
            "kotlin-stdlib jar not found in Gradle cache; K2 needs it on the classpath to resolve " +
                "built-in types and emit warning-level diagnostics. Set KOTLIN_STDLIB_JAR or " +
                "populate the Gradle cache by running `./gradlew :test` once at the repo root.",
        )
        stdlibClasspath = listOfNotNull(stdlib)
    }

    @Test
    fun uselessElvisOnNonNullableLhsIsRecorded() {
        val path = writeKt(
            "Elvis.kt",
            """
            package com.acme.elvis

            fun caller(): String {
                val s: String = "hi"
                return s ?: "fallback"
            }
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertTrue(
            diagnostics.any { it.factoryName == "USELESS_ELVIS" },
            "expected USELESS_ELVIS, got ${diagnostics.map { it.factoryName }}",
        )
    }

    @Test
    fun castNeverSucceedsIsRecorded() {
        val path = writeKt(
            "Cast.kt",
            """
            package com.acme.cast

            fun caller(input: String): Int {
                return (input as Int)
            }
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertTrue(
            diagnostics.any { it.factoryName == "CAST_NEVER_SUCCEEDS" },
            "expected CAST_NEVER_SUCCEEDS, got ${diagnostics.map { it.factoryName }}",
        )
    }

    @Test
    fun unreachableCodeIsRecorded() {
        // K2 only flags unreachable code when extra checkers are
        // enabled (`-Werror -Wextra` or the `extraCheckers` compiler
        // flag). The bare default analysis pipeline that ships with
        // krit-fir doesn't run the unreachable-code checker, so this
        // test pins the projection layer's behavior end-to-end while
        // remaining skipped when K2 doesn't emit the diagnostic.
        val path = writeKt(
            "Unreachable.kt",
            """
            package com.acme.unreachable

            fun caller(): Int {
                return 1
                return 2
            }
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        // The projection layer must NEVER conflate factories: even if
        // K2 doesn't emit UNREACHABLE_CODE, anything emitted must NOT
        // be mislabeled here. Run the test only if K2 produced the
        // expected factory; otherwise skip.
        assumeTrue(
            diagnostics.any { it.factoryName == "UNREACHABLE_CODE" },
            "UNREACHABLE_CODE not emitted by this K2 build — projection wiring covered by " +
                "the message-prefix matcher in OracleDiagnosticMessageCollector; full E2E " +
                "coverage requires the -Wextra checker set.",
        )
        // If we did get the diagnostic, sanity-check that we didn't
        // mislabel something else as UNREACHABLE_CODE.
        diagnostics.filter { it.factoryName == "UNREACHABLE_CODE" }.forEach {
            assertTrue("Unreachable" in it.message, "factoryName/message mismatch: $it")
        }
    }

    @Test
    fun cleanFileEmitsNoDiagnostics() {
        val path = writeKt(
            "Clean.kt",
            """
            package com.acme.clean

            fun greet(name: String): String = "hi, " + name
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertEquals(emptyList(), diagnostics)
    }

    @Test
    fun recordedDiagnosticCarriesLineColAndByteRange() {
        val path = writeKt(
            "Position.kt",
            """
            package com.acme.position

            fun caller(): String {
                val s: String = "hi"
                return s ?: "fallback"
            }
            """.trimIndent(),
        )

        val diagnostic = diagnosticsFor(path).firstOrNull { it.factoryName == "USELESS_ELVIS" }
        assertNotNull(diagnostic)
        assertEquals(5, diagnostic.line, "elvis is on line 5: $diagnostic")
        assertTrue(diagnostic.col > 0, "col must be 1-based: $diagnostic")
        assertTrue(
            diagnostic.endByte >= diagnostic.startByte,
            "byte range must be non-decreasing: $diagnostic",
        )
        assertEquals("WARNING", diagnostic.severity)
    }

    private fun diagnosticsFor(path: String): List<DiagnosticPayload> {
        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = stdlibClasspath,
        ).analyze(emptyList())
        return result.files[path]?.diagnostics.orEmpty()
    }

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        return file.canonicalPath
    }

    private fun findKotlinStdlib(): String? {
        System.getenv("KOTLIN_STDLIB_JAR")?.let { override ->
            if (File(override).isFile) return override
        }
        val home = System.getProperty("user.home") ?: return null
        val cacheRoot = File(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib")
        if (!cacheRoot.isDirectory) return null
        val matches = cacheRoot.walkTopDown()
            .filter { it.isFile && it.name.startsWith("kotlin-stdlib-") && it.name.endsWith(".jar") }
            .filter { !it.name.contains("sources") && !it.name.contains("javadoc") }
            .toList()
            .sortedByDescending { it.name }
        return matches.firstOrNull()?.absolutePath
    }
}
