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
 * [`AnalysisSession.analyze`]: the retained K2 warning factories round-trip
 * through the [`OracleDiagnosticMessageCollector`] into each
 * [`FilePayload.diagnostics`] list.
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
        // K2's UNREACHABLE_CODE checker lives in the experimental
        // checker set. KritFirCheckers registers it with the plugin's
        // control-flow checkers so the projection collects it on the
        // default analyze path.
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
        assertTrue(
            diagnostics.any { it.factoryName == "UNREACHABLE_CODE" },
            "expected UNREACHABLE_CODE, got ${diagnostics.map { it.factoryName }}",
        )
        // Belt-and-suspenders: ensure factoryName ↔ message pairing
        // isn't crossed (a prefix-match miss would otherwise label
        // an unrelated warning with the wrong factory).
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

    @Test
    fun deprecationIsRecordedForEveryReferenceKindAndOnlyTheDeprecatedOverload() {
        writeKt(
            "Lib.kt",
            """
            package com.acme.dep

            @Deprecated("use newFn") fun oldFn() {}
            fun overloaded(i: Int) {}
            @Deprecated("use the Int overload") fun overloaded(s: String) {}
            @Deprecated("use NewType") class OldType
            class Holder {
                @Deprecated("use newProp") val oldProp: Int = 1
                val newProp: Int = 2
            }
            open class Base { @Deprecated("gone") open fun m() {} }
            class Child : Base()
            @Deprecated("use String") typealias OldAlias = String
            """.trimIndent(),
        )
        // Deliberately free of the tokens the old lexical gate keyed on.
        val use = writeKt(
            "Use.kt",
            """
            package com.acme.dep

            import java.util.Date

            fun useAll(h: Holder, c: Child, t: OldType, a: OldAlias) {
                oldFn()
                overloaded(1)
                overloaded("s")
                println(h.oldProp)
                println(h.newProp)
                c.m()
                println(Date().year)
            }
            """.trimIndent(),
        )

        val deprecations = diagnosticsFor(use).filter { it.factoryName == "DEPRECATION" }
        // Line 5: the OldType and OldAlias parameter types. Then oldFn (6), the
        // String overload (8), oldProp (9), the inherited m (11), and the
        // Java-deprecated Date.year getter (12). Not overloaded(1) (7) or
        // newProp (10).
        assertEquals(
            listOf(5, 5, 6, 8, 9, 11, 12),
            deprecations.map { it.line }.sorted(),
            "got ${deprecations.map { "${it.line}:${it.col} ${it.message}" }}",
        )
        assertTrue(deprecations.all { it.severity == "WARNING" })
    }

    @Test
    fun deprecationMessageContainingBracketedTextIsStillRecorded() {
        // The @Deprecated message is embedded in the compiler message; an
        // unanchored "[NAME]" check would mistake it for a krit plugin
        // diagnostic and drop it.
        val path = writeKt(
            "Bracketed.kt",
            """
            package com.acme.bracketed

            @Deprecated("[OLD] use g") fun f() {}
            fun g() {}
            fun caller() { f() }
            """.trimIndent(),
        )

        val deprecations = diagnosticsFor(path).filter { it.factoryName == "DEPRECATION" }
        assertEquals(listOf(5), deprecations.map { it.line }, "got $deprecations")
    }

    @Test
    fun otherDeprecationWordedWarningsAreNotRecordedAsDeprecation() {
        // DEPRECATED_IDENTITY_EQUALS renders "Identity equality … is
        // deprecated." — a different factory the DEPRECATION pattern must not
        // claim.
        val path = writeKt(
            "Identity.kt",
            """
            package com.acme.identity

            fun same(a: Int, b: Int) = a === b
            """.trimIndent(),
        )

        assertEquals(emptyList(), diagnosticsFor(path).filter { it.factoryName == "DEPRECATION" })
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
        return file.absolutePath
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
