package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.oracle.ExpressionPayload
import dev.jasonpearson.krit.fir.oracle.FilePayload
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for the [`OracleExpressionChecker`] call
 * projection: every visited [`FirFunctionCall`] should surface in the
 * resulting `expressions` map keyed by `"line:col"` with the resolved
 * callable FQN, suspend flag, and callable annotations.
 */
class AnalysisSessionExpressionsTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun resolvedCallsRecordFqnSuspendFlagAndByteOffsets() {
        val path = writeKt(
            "Calls.kt",
            """
            package com.acme.calls

            suspend fun runSuspend(): Int = 1
            fun runPlain(): Int = 2

            suspend fun caller() {
                runSuspend()
                runPlain()
            }
            """.trimIndent(),
        )

        val expressions = expressionsFor(path)
        val resolved = expressions.values.associateBy { it.callTarget }

        val suspendCall = resolved["com.acme.calls.runSuspend"]
        assertNotNull(suspendCall, "runSuspend missing: ${expressions.values.map { it.callTarget }}")
        assertEquals(true, suspendCall.callTargetResolved)
        assertEquals(true, suspendCall.callTargetSuspend)
        assertTrue(suspendCall.endByte > suspendCall.startByte, "byte range empty: $suspendCall")

        val plainCall = resolved["com.acme.calls.runPlain"]
        assertNotNull(plainCall)
        assertEquals(true, plainCall.callTargetResolved)
        assertEquals(false, plainCall.callTargetSuspend)
    }

    @Test
    fun keysUseOneBasedLineAndColumnAtTheCallSite() {
        val path = writeKt(
            "Position.kt",
            """
            package com.acme.position

            fun target(): Int = 1

            fun caller() {
                target()
            }
            """.trimIndent(),
        )

        val expressions = expressionsFor(path)
        // The single `target()` call site sits on line 6, column 5 of
        // the formatted source above.
        val entry = expressions.entries.singleOrNull { it.value.callTarget == "com.acme.position.target" }
        assertNotNull(entry, "target() call missing: ${expressions.entries}")
        assertEquals("6:5", entry.key)
    }

    @Test
    fun annotationsOnCalleeSurfaceAsFqnList() {
        val path = writeKt(
            "Annotated.kt",
            """
            package com.acme.annotated

            @Target(AnnotationTarget.FUNCTION)
            @Retention(AnnotationRetention.SOURCE)
            annotation class Marker

            @Marker
            fun target(): Int = 1

            fun caller() {
                target()
            }
            """.trimIndent(),
        )

        val expressions = expressionsFor(path)
        val targetCall = expressions.values.singleOrNull { it.callTarget == "com.acme.annotated.target" }
        assertNotNull(targetCall, "target() call missing: ${expressions.values.map { it.callTarget }}")
        assertTrue(
            "com.acme.annotated.Marker" in targetCall.annotations,
            "expected Marker annotation, got ${targetCall.annotations}",
        )
    }

    @Test
    fun distinctCallSitesProduceDistinctKeys() {
        val path = writeKt(
            "Many.kt",
            """
            package com.acme.many

            fun target(): Int = 1

            fun caller() {
                target()
                target()
                target()
            }
            """.trimIndent(),
        )

        val expressions = expressionsFor(path)
        val targetCalls = expressions.entries.filter { it.value.callTarget == "com.acme.many.target" }
        assertEquals(3, targetCalls.size, "expected three distinct target calls, got $targetCalls")
        assertEquals(
            targetCalls.size,
            targetCalls.map { it.key }.toSet().size,
            "expected unique line:col keys for each call",
        )
    }

    private fun expressionsFor(path: String): Map<String, ExpressionPayload> {
        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())
        val file: FilePayload? = result.files[path]
        assertNotNull(file, "file payload missing: files=${result.files.keys}")
        return file.expressions
    }

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        // K2's source-file path goes through canonicalization (macOS
        // symlinks `/var` → `/private/var`); return the canonical path
        // so test assertions against the `files` map line up with what
        // the projection layer captured.
        return file.canonicalPath
    }
}
