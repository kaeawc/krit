package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// SerialVersionUIDInSerializableClass cases a single-file golden cannot
// express: a same-named Serializable class in another file, and a Java
// source base class.
class SerialVersionUIDInSerializableClassCrossFileTest {

    private val ruleId = "SerialVersionUIDInSerializableClass"

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(sources, FirRuleCompileContext(enabledRuleIds = setOf(ruleId)))
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == ruleId }.map { it.file to it.line }
            .sortedWith(compareBy({ it.first }, { it.second }))
    }

    // Go resolves `Error` by simple name to model.Resource.Error, declared in
    // another file and Serializable, and reports FatalFailure. The class
    // really extends kotlin.Error, which is Serializable through Throwable,
    // so the finding is true and FIR keeps it. The plain exception on line 5
    // reports in neither.
    @Test fun exceptionNamedLikeASerializableClassInAnotherFile() {
        val sources = mapOf(
            "Resource.kt" to """
                package model

                sealed class Resource : java.io.Serializable {
                    companion object {
                        private const val serialVersionUID = 1L
                    }

                    data class Error(val message: String) : Resource()
                }
            """.trimIndent(),
            "Failures.kt" to """
                package app

                class FatalFailure(msg: String) : kotlin.Error(msg)

                class Plain(msg: String) : RuntimeException(msg)
            """.trimIndent(),
        )
        assertEquals(listOf("Failures.kt" to 3, "Resource.kt" to 8), findings(sources))
    }

    // Divergence (recall): ExBase is a Java source class implementing
    // Serializable. Go's resolver indexes only Kotlin sources, so it misses
    // ExDerived; the class is Serializable and lacks the field, so FIR
    // reports it.
    @Test fun serializableThroughAJavaSourceClass() {
        val sources = mapOf(
            "javabase/ExBase.java" to """
                package javabase;

                public class ExBase implements java.io.Serializable {
                    private static final long serialVersionUID = 1L;
                }
            """.trimIndent(),
            "Derived.kt" to """
                package app

                import javabase.ExBase

                class ExDerived : ExBase()
            """.trimIndent(),
        )
        assertEquals(listOf("Derived.kt" to 5), findings(sources))
    }
}
