package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UseSparseArrays cases a single-file golden cannot express: key and value
// types imported from another package under the names Long and Boolean.
class UseSparseArraysCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("UseSparseArrays")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "UseSparseArrays" }.map { it.file to it.line }
    }

    private val other = "Other.kt" to """
        package other

        class Long
        class Boolean
    """.trimIndent()

    // Divergence (precision): `import other.Long` makes the key other.Long,
    // not kotlin.Long. Go reads the key by name and reports LongSparseArray
    // on line 5. FIR is correct to drop it: the message ("HashMap<Long, ...>")
    // is false for this code. The kotlin.Long key on line 7 still reports.
    @Test fun importedKeyLookalike() {
        val sources = mapOf(
            other,
            "User.kt" to """
                package demo

                import other.Long

                fun f() = HashMap<Long, String>()

                fun g() = HashMap<kotlin.Long, String>()
            """.trimIndent(),
        )
        assertEquals(listOf("User.kt" to 7), findings(sources))
    }

    // Divergence (message only): the key is kotlin.Int, so both report line 5,
    // but the value is other.Boolean. Go reads the value's name `Boolean` and
    // suggests SparseBooleanArray; FIR suggests SparseArray, which is correct.
    // The merge keeps Go's message because the finding is on the same line.
    @Test fun importedValueLookalike() {
        val sources = mapOf(
            other,
            "User.kt" to """
                package demo

                import other.Boolean

                fun f() = HashMap<kotlin.Int, Boolean>()
            """.trimIndent(),
        )
        assertEquals(listOf("User.kt" to 5), findings(sources))
    }
}
