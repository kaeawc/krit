package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import org.junit.jupiter.api.io.TempDir

class FirDefinitelyNonNullTest {
    @TempDir lateinit var tmp: Path

    @Test
    fun onlyProvenNonNullOperandsPass() {
        val file = tmp.resolve("NonNull.kt").toFile().apply {
            writeText("""
                fun nonNull(value: String) = value ?: "fallback"
                fun nullable(value: String?) = value ?: "fallback"
                fun platform() = System.getenv("KRIT_VALUE") ?: "fallback"
                fun <T> unbounded(value: T, fallback: T): T = value ?: fallback
                fun <T : Any> bounded(value: T, fallback: T): T = value ?: fallback
                fun <T> definitely(value: T & Any, fallback: T & Any): T & Any = value ?: fallback
                fun smart(value: String?): String {
                    if (value == null) return "missing"
                    return value ?: "fallback"
                }
            """.trimIndent())
        }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(1, listOf(FileRef(file.absolutePath)), setOf("DefinitelyNonNullProbe"), emptyMap())
        val actual = result.findings.filter { it.rule == "DefinitelyNonNullProbe" }.map { it.message }
        assertEquals(listOf("true", "false", "false", "false", "true", "true", "true"), actual)
    }
}
