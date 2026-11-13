package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UseSparseArrays cases a single-file golden cannot express: a type argument
// imported from another package under the name Int or Boolean.
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

        class Int
        class Boolean
    """.trimIndent()

    // Divergence (precision): `import other.Int` makes the argument other.Int,
    // not kotlin.Int. Go reads the argument by name and reports line 6. FIR is
    // correct to drop it: the message ("SparseIntArray") is false for this
    // code. The kotlin.Int argument on line 8 still reports.
    @Test fun importedArgumentLookalike() {
        val sources = mapOf(
            other,
            "User.kt" to """
                package demo

                import android.util.SparseArray
                import other.Int

                fun f() = SparseArray<Int>()

                fun g() = SparseArray<kotlin.Int>()
            """.trimIndent(),
        )
        assertEquals(listOf("User.kt" to 8), findings(sources))
    }

    // Divergence (precision): the same for Boolean.
    @Test fun importedBooleanLookalike() {
        val sources = mapOf(
            other,
            "User.kt" to """
                package demo

                import android.util.SparseArray
                import other.Boolean

                fun f() = SparseArray<Boolean>()
            """.trimIndent(),
        )
        assertEquals(emptyList(), findings(sources))
    }
}
