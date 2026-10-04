package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// DoubleMutabilityForCollection cases a single-file golden cannot express: a
// type named like a built-in collection declared in another file of the same
// package. Go looks for a shadowing declaration only in the file it checks,
// so it reads the name as the built-in collection.
class DoubleMutabilityForCollectionCrossFileTest {

    private val rule = "DoubleMutabilityForCollection"

    // File -> finding lines.
    private fun findings(sources: Map<String, String>): Map<String, List<Int>> {
        val result = KritFirProbe.compile(
            sources.mapValues { it.value.trimIndent() },
            FirRuleCompileContext(enabledRuleIds = setOf(rule)),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == rule }
            .groupBy({ it.file }, { it.line })
            .mapValues { it.value.sorted() }
    }

    // Like Go: p.LinkedHashMap is a java.util.LinkedHashMap subclass, so the
    // property's type is a mutable map and Go's finding (line 6) is true.
    @Test fun sameNamedMutableSubclassInAnotherFile() {
        val sources = mapOf(
            "Types.kt" to """
                package p

                class LinkedHashMap<K, V> : java.util.LinkedHashMap<K, V>()
            """,
            "Probe.kt" to """
                package p

                fun <K, V> cache(): LinkedHashMap<K, V> = LinkedHashMap()

                class Holder {
                    var sameFile: LinkedHashMap<String, Int> = cache()
                }
            """,
        )
        assertEquals(mapOf("Probe.kt" to listOf(6)), findings(sources))
    }

    // Like Go: q.HashSet is a type alias of CopyOnWriteArraySet, a mutable set.
    @Test fun sameNamedMutableTypeAliasInAnotherFile() {
        val sources = mapOf(
            "Aliases.kt" to """
                package q

                typealias HashSet<T> = java.util.concurrent.CopyOnWriteArraySet<T>
            """,
            "Use.kt" to """
                package q

                fun <T> fresh(): HashSet<T> = HashSet()

                class Holder {
                    var set: HashSet<String> = fresh()
                }
            """,
        )
        assertEquals(mapOf("Use.kt" to listOf(6)), findings(sources))
    }

    // Divergence: Go reports line 4 (the written name MutableList, with no
    // same-named declaration in Use.kt), but r.MutableList is not a
    // collection, so the message is false of the code.
    @Test fun sameNamedNonCollectionInAnotherFile() {
        val sources = mapOf(
            "Types.kt" to """
                package r

                class MutableList
            """,
            "Use.kt" to """
                package r

                class Holder {
                    var list: MutableList = MutableList()
                }
            """,
        )
        assertEquals(emptyMap(), findings(sources))
    }
}
