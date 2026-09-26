package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UseEmptyCounterpart cases a single-file golden cannot express: project
// lookalikes of the stdlib factories imported from another package.
class UseEmptyCounterpartCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("UseEmptyCounterpart")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "UseEmptyCounterpart" }.map { it.file to it.line }
    }

    private val util = "Util.kt" to """
        package util

        fun <T> listOf(): List<T> = ArrayList()

        object sequenceOf {
            operator fun invoke(): Sequence<Int> = generateSequence { 1 }
        }
    """.trimIndent()

    // Divergence: `import util.listOf` and `import util.sequenceOf` shadow the
    // default-imported stdlib factories, so lines 6 and 7 call the project
    // declarations, not the factories. Go reports both by their written names;
    // the empty counterpart is not a replacement for them. The qualified stdlib
    // call on line 8 still reports (Go misses it).
    @Test fun importedLookalikes() {
        val sources = mapOf(
            util,
            "User.kt" to """
                package demo

                import util.listOf
                import util.sequenceOf

                fun f(): List<String> = listOf()
                fun g() = sequenceOf()
                fun h() = kotlin.collections.listOf<String>()
            """.trimIndent(),
        )
        assertEquals(listOf("User.kt" to 8), findings(sources))
    }
}
