package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import org.junit.jupiter.api.io.TempDir

class FirTypeAnnotationsTest {
    @TempDir lateinit var tmp: Path

    @Test
    fun annotationsResolveByClassIdAndSurviveTypeAliasesAndNullability() {
        val source = """
            package sample
            @Target(AnnotationTarget.TYPE)
            annotation class Marker
            @Target(AnnotationTarget.TYPE)
            annotation class Other
            typealias Callback = @Marker () -> Unit
            val plain: () -> Unit = {}
            val annotated: @Marker () -> Unit = {}
            val aliased: Callback = {}
            val nullable: Callback? = null
            val unrelated: @Other () -> Unit = {}
        """.trimIndent()
        val file = tmp.resolve("Sample.kt").toFile().apply { writeText(source) }
        val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(1, listOf(FileRef(file.absolutePath)), setOf("TypeAnnotationProbe"), emptyMap())
        val found = result.findings.filter { it.rule == "TypeAnnotationProbe" }
            .associate { it.message.substringBefore(' ') to it.message.substringAfter(' ') }
        assertEquals(mapOf("plain" to "false", "annotated" to "true", "aliased" to "true", "nullable" to "true", "unrelated" to "false"), found)
    }
}
