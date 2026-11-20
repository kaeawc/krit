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
            private var list: MutableList<String> = buildList()
            private var map: MutableMap<String, Int> = buildMap()
            private var factoryMap = mutableMapOf<String, Int>()
            private var collection: MutableCollection<String> = buildCollection()
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

        private var exampleBag: com.example.Bag = com.example.Bag()
        private var otherBag: other.Bag = other.Bag()
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

            private var aliased: Names = names()
        """
        assertEquals(
            mapOf("Use.kt" to mapOf(7 to 1)),
            findings(use, mapOf("mutableTypes" to listOf("Names"))),
        )
    }

    @Test fun requestListedTestFileIsSkipped() {
        val use = """
            package use

            private var list = mutableListOf<String>()
        """
        assertEquals(mapOf("Use.kt" to mapOf(3 to 1)), findings(use))
        assertEquals(emptyMap(), findings(use, testFiles = setOf("Use.kt")))
    }

    @Test fun onlyValCandidatesControlsAllExemptions() {
        val use = """
            package use
            class Cache {
                var publicItems: MutableList<String> = mutableListOf()
                internal var shared: MutableSet<String> = mutableSetOf()
                protected var inherited: MutableMap<String, Int> = mutableMapOf()
                private var nullable: MutableList<String>? = null
                private var written = mutableListOf<String>()
                private var candidate = mutableListOf<String>()
                fun reset() { written = mutableListOf() }
                fun run() {
                    var local: MutableList<String> = mutableListOf()
                    local = mutableListOf()
                }
            }
        """
        assertEquals(mapOf("Use.kt" to mapOf(8 to 1)), findings(use))
        assertEquals(mapOf("Use.kt" to mapOf(8 to 1)), findings(use, mapOf("onlyValCandidates" to true)))
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1, 4 to 1, 5 to 1, 6 to 1, 7 to 1, 8 to 1, 11 to 1)),
            findings(use, mapOf("onlyValCandidates" to false)),
        )
    }

    @Test fun compatibilityIncludesTestFilesButNotConstructorParameters() {
        val use = """
            package use
            class Cache(var publicItems: MutableList<String>, private var privateItems: MutableList<String>)
            private var items = mutableListOf<String>()
        """
        assertEquals(mapOf("Use.kt" to mapOf(2 to 1, 3 to 1)), findings(use))
        assertEquals(emptyMap(), findings(use, testFiles = setOf("Use.kt")))
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1)),
            findings(use, mapOf("onlyValCandidates" to false), testFiles = setOf("Use.kt")),
        )
    }

    @Test fun compatibilityMatchesResolvedTypesNotFactoryNames() {
        val use = """
            package use
            private var readOnly: List<String> = mutableListOf()
            private var collection: MutableCollection<String> = mutableListOf()
            fun mutableSetOf(): Int = 0
            private var count = mutableSetOf()
            private var actual = kotlin.collections.mutableListOf<String>()
        """
        assertEquals(
            mapOf("Use.kt" to mapOf(6 to 1)),
            findings(use, mapOf("onlyValCandidates" to false)),
        )
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1)),
            findings(use, mapOf("onlyValCandidates" to false, "mutableTypes" to listOf("kotlin.collections.MutableCollection"))),
        )
    }

    @Test fun compatibilityMatchesQualifiedCustomTypesExactly() {
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1)),
            findings(bags, mapOf("onlyValCandidates" to false, "mutableTypes" to listOf("com.example.Bag"))),
        )
        assertEquals(
            emptyMap(),
            findings("private var items = mutableListOf<String>()", mapOf("onlyValCandidates" to false, "mutableTypes" to listOf("MutableList"))),
        )
    }

    @Test fun compatibilityMatchesExpandedAliases() {
        val use = """
            package use
            typealias Items = MutableList<String>
            private var items: Items = mutableListOf()
        """
        assertEquals(mapOf("Use.kt" to mapOf(3 to 1)), findings(use, mapOf("onlyValCandidates" to false)))
        assertEquals(
            emptyMap(),
            findings(use, mapOf("onlyValCandidates" to false, "mutableTypes" to listOf("use.Items"))),
        )
    }

    // Paired with the Go compatibilitySourceAnalysisDivergences case: FIR expands
    // the qualified alias and resolves toMutableList(); Go reports only line 5.
    @Test fun compatibilitySourceAnalysisDivergences() {
        val use = """
            package use
            typealias Items = MutableList<String>
            var qualified: use.Items = mutableListOf()
            var converted = listOf("item").toMutableList()
            var factory = mutableListOf<String>()
            var readOnly: List<String> = mutableListOf()
        """
        assertEquals(
            mapOf("Use.kt" to mapOf(3 to 1, 4 to 1, 5 to 1)),
            findings(use, mapOf("onlyValCandidates" to false)),
        )
    }

    @Test fun compatibilityExtensionUsesDeclaredTypeNotReceiver() {
        val use = """
            package use
            var MutableMap<String, String>.contentType: String?
                get() = this["Content-Type"]
                set(value) { if (value != null) this["Content-Type"] = value }
        """
        assertEquals(emptyMap(), findings(use, mapOf("onlyValCandidates" to false)))
    }

    @Test fun compatibilityHandlesQualifiedTypesAndFactoryImports() {
        val use = """
            package use
            import kotlin.collections.mutableListOf
            class ArrayList
            private var imported = mutableListOf<String>()
            private var constructed = java.util.ArrayList<String>()
            private var qualified: kotlin.collections.MutableList<String> = mutableListOf()
            private var javaList: java.util.ArrayList<String> = java.util.ArrayList()
            private var nullable: java.util.HashMap<String, Int>? = null
        """
        assertEquals(
            mapOf("Use.kt" to mapOf(4 to 1, 5 to 1, 6 to 1, 7 to 1, 8 to 1)),
            findings(use, mapOf("onlyValCandidates" to false)),
        )
        assertEquals(
            mapOf("Use.kt" to mapOf(4 to 1, 5 to 1, 6 to 1, 7 to 1)),
            findings(use, mapOf("onlyValCandidates" to true)),
        )
    }
}
