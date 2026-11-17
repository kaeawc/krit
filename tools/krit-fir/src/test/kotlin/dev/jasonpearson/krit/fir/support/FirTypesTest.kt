package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import org.junit.jupiter.api.io.TempDir

/** Runs the expanded-type helpers through a real K2 compile via the test-only ExpandedTypeProbe rule. */
class FirTypesTest {
    @TempDir lateinit var tmp: Path

    private fun probe(source: String): Map<String, String> {
        val file = tmp.resolve("Sample.kt").toFile().apply { writeText(source) }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(1, listOf(FileRef(file.absolutePath)), setOf("ExpandedTypeProbe"), emptyMap())
        return result.findings.filter { it.rule == "ExpandedTypeProbe" }
            .associate { it.message.substringBefore(' ') to it.message.substringAfter(' ') }
    }

    private val sample = """
        typealias Names = List<String>
        typealias Alias2 = Names
        class Box<T>(val value: T) {
            val param: T = value
        }
        class Holder<T : Any>(t: T) {
            val definitely: T & Any = t
        }
        val plain: String = ""
        val aliased: Names = emptyList()
        val chained: Alias2 = emptyList()
        val nullableAlias: Names? = null
        val platform: java.lang.StringBuilder = java.lang.StringBuilder()
        val lambda: () -> Unit = {}
    """.trimIndent()

    @Test
    fun classTypeIsReturnedForPlainAndAliasedTypes() {
        val found = probe(sample)
        assertEquals("class=kotlin/String bound=Class nullable=false", found["plain"])
        assertEquals("class=kotlin/collections/List bound=Class nullable=false", found["aliased"])
        assertEquals("class=kotlin/collections/List bound=Class nullable=false", found["chained"])
    }

    @Test
    fun nullabilityIsKeptThroughAliasExpansion() {
        assertEquals("class=kotlin/collections/List bound=Class nullable=true", probe(sample)["nullableAlias"])
    }

    @Test
    fun typeParameterHasNoClassType() {
        assertEquals("class=none bound=Other nullable=false", probe(sample)["param"])
    }

    @Test
    fun definitelyNotNullTypeHasNoClassType() {
        assertEquals("class=none bound=Other nullable=false", probe(sample)["definitely"])
    }

    @Test
    fun javaClassTypeIsReturnedAsClass() {
        assertEquals("class=java/lang/StringBuilder bound=Class nullable=false", probe(sample)["platform"])
    }

    @Test
    fun functionTypeIsAClassType() {
        assertEquals("class=kotlin/Function0 bound=Class nullable=false", probe(sample)["lambda"])
    }
}
