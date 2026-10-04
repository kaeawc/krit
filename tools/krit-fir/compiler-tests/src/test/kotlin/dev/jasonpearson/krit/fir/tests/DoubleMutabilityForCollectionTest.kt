package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// DoubleMutabilityForCollection's mutableTypes option and test-file skip,
// which the golden data (default config, no check request) cannot exercise.
class DoubleMutabilityForCollectionTest {

    private val rule = "DoubleMutabilityForCollection"

    private val library = mapOf(
        "Bags.kt" to """
            package com.example

            class Bag

            fun <T> buildList(): MutableList<T> = mutableListOf()
            fun <K, V> buildMap(): MutableMap<K, V> = mutableMapOf()
            fun <T> buildCollection(): MutableCollection<T> = mutableListOf()
        """.trimIndent(),
        "OtherBag.kt" to """
            package other

            class Bag
        """.trimIndent(),
    )

    // Line -> finding count in Use.kt.
    private fun findings(
        use: String,
        options: Map<String, Any?>? = null,
        testFiles: Set<String> = emptySet(),
    ): Map<String, Map<Int, Int>> {
        val sources = library + ("Use.kt" to use.trimIndent())
        val configs = if (options == null) emptyMap() else mapOf(rule to options)
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf(rule), ruleConfigs = configs),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == rule }
            .groupBy { it.file }
            .mapValues { (_, diags) -> diags.groupingBy { it.line }.eachCount() }
    }

    private val configured = """
        package use

        import com.example.buildCollection
        import com.example.buildList
        import com.example.buildMap

        class Holder {
            var list: MutableList<String> = buildList()
            var map: MutableMap<String, Int> = buildMap()
            var factoryMap = mutableMapOf<String, Int>()
            var collection: MutableCollection<String> = buildCollection()
        }
    """

    @Test fun defaultConfigListsNoMutableCollection() {
        assertEquals(mapOf("Use.kt" to mapOf(8 to 1, 9 to 1, 10 to 1)), findings(configured))
    }

    @Test fun configuredTypesLimitDeclaredTypesButNotFactories() {
        // Go reports `var factoryMap = mutableMapOf()` through its factory-name
        // fallback whatever mutableTypes lists.
        assertEquals(
            mapOf("Use.kt" to mapOf(8 to 1, 10 to 1)),
            findings(configured, mapOf("mutableTypes" to listOf("kotlin.collections.MutableList"))),
        )
    }

    @Test fun emptyOptionFallsBackToTheGoStructDefault() {
        // Go's defaultDoubleMutableTypes, which also lists MutableCollection.
        assertEquals(
            mapOf("Use.kt" to mapOf(8 to 1, 9 to 1, 10 to 1, 11 to 1)),
            findings(configured, mapOf("mutableTypes" to emptyList<String>())),
        )
    }

    private val bags = """
        package use

        var exampleBag: com.example.Bag = com.example.Bag()
        var otherBag: other.Bag = other.Bag()
    """

    @Test fun qualifiedCustomTypeMatchesThatClass() {
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1)),
            findings(bags, mapOf("mutableTypes" to listOf("com.example.Bag"))),
        )
    }

    @Test fun simpleCustomTypeMatchesAnyClassOfThatName() {
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1, 4 to 1)),
            findings(bags, mapOf("mutableTypes" to listOf("Bag"))),
        )
    }

    // Like Go, which matches the written simple name `Names`: K2 keeps the
    // alias in the declared type, and the checker matches the written name as
    // well as the expansion.
    @Test fun customTypeAliasEntryMatchesTheWrittenAlias() {
        val use = """
            package use

            typealias Names = MutableList<String>

            fun names(): Names = mutableListOf()

            var aliased: Names = names()
        """
        assertEquals(
            mapOf("Use.kt" to mapOf(7 to 1)),
            findings(use, mapOf("mutableTypes" to listOf("Names"))),
        )
    }

    @Test fun requestListedTestFileIsSkipped() {
        val use = """
            package use

            var list = mutableListOf<String>()
        """
        assertEquals(mapOf("Use.kt" to mapOf(3 to 1)), findings(use))
        assertEquals(emptyMap(), findings(use, testFiles = setOf("Use.kt")))
    }
}
