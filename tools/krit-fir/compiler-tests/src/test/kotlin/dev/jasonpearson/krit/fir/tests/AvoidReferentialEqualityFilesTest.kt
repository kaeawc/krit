package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class AvoidReferentialEqualityFilesTest {
    private val rule = "AvoidReferentialEquality"

    private fun findings(sources: Map<String, String>, patterns: Any? = listOf("kotlin.String")): Int {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(
                enabledRuleIds = setOf(rule),
                ruleConfigs = mapOf(rule to mapOf("forbiddenTypePatterns" to patterns)),
            ),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == rule }
    }

    @Test fun importedLookalikeIsNotKotlinString() {
        assertEquals(0, findings(mapOf(
            "Types.kt" to "package other; class String",
            "Use.kt" to "package sample; import other.String; fun same(a: String, b: String) = a === b",
        )))
    }

    @Test fun importedAliasExpandsToKotlinString() {
        assertEquals(1, findings(mapOf(
            "Types.kt" to "package other; typealias Text = String",
            "Use.kt" to "package sample; import other.Text; fun same(a: Text, b: Text) = a === b",
        )))
    }

    @Test fun configuredPatternsAreFullFqnGlobs() {
        val sources = mapOf("Use.kt" to "package sample; class Value; fun same(a: Value?, b: Value?) = a !== b")
        assertEquals(0, findings(sources))
        assertEquals(1, findings(sources, listOf("sample.Value")))
        assertEquals(1, findings(sources, listOf("sample.V?l*")))
        assertEquals(0, findings(sources, listOf("Value")))
        assertEquals(0, findings(sources, emptyList<String>()))
        assertEquals(0, findings(sources, listOf(" ")))
        assertEquals(0, findings(sources, listOf("sample.[V]alue")))
    }

    @Test fun primitiveVarargsUsePrimitiveArrayTypes() {
        val intVararg = mapOf("Use.kt" to "package sample; fun same(vararg values: Int, other: Any) = values === other")
        assertEquals(1, findings(intVararg, listOf("kotlin.IntArray")))
        assertEquals(0, findings(intVararg, listOf("kotlin.Array")))

        val stringVararg = mapOf("Use.kt" to "package sample; fun same(vararg values: String, other: Any) = values === other")
        assertEquals(1, findings(stringVararg, listOf("kotlin.Array")))
        assertEquals(0, findings(stringVararg, listOf("kotlin.String")))
    }

    @Test fun scalarAndReasonedPatternsMatchGoOptions() {
        val strings = mapOf("Use.kt" to "package sample; fun same(a: String, b: String) = a === b")
        assertEquals(1, findings(strings, "kotlin.String"))
        assertEquals(0, findings(strings, "kotlin.Int"))
        assertEquals(1, findings(strings, listOf(mapOf("value" to "kotlin.String", "reason" to "value semantics"))))
        assertEquals(0, findings(strings, emptyList<String>()))
        assertEquals(1, findings(strings, null))
        assertEquals(1, findings(strings, 42))
        assertEquals(1, findings(strings, mapOf("value" to "kotlin.Int")))
    }

    @Test fun reportsTheGoMessageOnTheOperandStartLine() {
        val result = KritFirProbe.compile(
            mapOf("Use.kt" to """
                package sample
                fun same(a: String, b: String): Boolean {
                    return a ===
                        b
                }
            """.trimIndent()),
            FirRuleCompileContext(enabledRuleIds = setOf(rule)),
        )
        assertTrue(result.clean, result.problems())
        val diagnostic = result.diags.single { it.name == rule }
        assertEquals(3, diagnostic.line)
        assertEquals(
            "Referential equality (===, !==) should be avoided. Use structural equality (==, !=) instead.",
            diagnostic.message.substringAfter("] "),
        )
    }

    @Test fun inheritedMembersDoNotUseConstructorOrTopLevelStrings() {
        assertEquals(0, findings(mapOf(
            "Types.kt" to "package sample; open class Entity(val id: Any); open class Base { val label: Any = Any() }; class Node { val label: Any = Any() }",
            "Use.kt" to """
                package sample
                val label = "default"
                fun fetch(): Any = Any()
                class Row(id: String) : Entity(id) { fun same(other: Any) = id === other }
                class GetterRow : Base() { val same get() = label === fetch() }
                fun make(candidate: Any) = object : Base() { fun same() = label === candidate }
                fun Node.same(candidate: Any) = label === candidate
                val Node.same: Boolean get() = label === fetch()
            """.trimIndent(),
        )))
    }

    @Test fun enumsAndObjectsKeepIdentitySemantics() {
        assertEquals(0, findings(mapOf("Use.kt" to """
            package sample
            enum class State { Ready, Done }
            object Sentinel
            fun enumIdentity(a: State, b: State) = a === b
            fun singleton(a: Any) = a === Sentinel
        """.trimIndent()), listOf("*")))
    }

    @Test fun cleanupGuardUsesTheComparedReceiver() {
        assertEquals(0, findings(mapOf("Use.kt" to """
            package sample
            class Resource { fun close() {} }
            fun release(a: Resource, b: Resource) {
                if (a !== b) a.close()
            }
        """.trimIndent()), listOf("sample.Resource")))
    }
}
