package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import org.junit.jupiter.api.io.TempDir

/** Runs isNonNothingSubtypeOf through a real K2 compile via the test-only SubtypeProbe rule. */
class FirSubtypingTest {
    @TempDir lateinit var tmp: Path

    private fun probe(source: String): Map<String, String> {
        val file = tmp.resolve("Sample.kt").toFile().apply { writeText(source) }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(1, listOf(FileRef(file.absolutePath)), setOf("SubtypeProbe"), emptyMap())
        return result.findings.filter { it.rule == "SubtypeProbe" }
            .associate { it.message.substringBefore(' ') to it.message.substringAfter(' ') }
    }

    private val sample = """
        val plain: String = ""
        val nullable: String? = null
        val unrelated: Int = 0
        val bottom: Nothing = TODO()
        val nullableBottom: Nothing? = null
        val inferredBottom = TODO()
    """.trimIndent()

    @Test
    fun realSubtypesMatch() {
        val found = probe(sample)
        assertEquals("plain=true guarded=true", found["plain"])
        assertEquals("plain=true guarded=true", found["nullable"])
    }

    @Test
    fun unrelatedTypeDoesNotMatch() {
        assertEquals("plain=false guarded=false", probe(sample)["unrelated"])
    }

    @Test
    fun nothingIsASubtypeOfEverythingButIsRejected() {
        val found = probe(sample)
        assertEquals("plain=true guarded=false", found["bottom"])
        assertEquals("plain=true guarded=false", found["nullableBottom"])
        assertEquals("plain=true guarded=false", found["inferredBottom"])
    }
}
