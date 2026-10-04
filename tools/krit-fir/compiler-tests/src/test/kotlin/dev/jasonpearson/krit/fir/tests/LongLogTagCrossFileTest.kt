package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// LongLogTag cases a single-file golden cannot express: a tag constant
// declared in another file.
class LongLogTagCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("LongLogTag")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "LongLogTag" }.map { it.file to it.line }
    }

    // Divergence (recall): Tags.LONG is declared in Tags.kt with a 27-character
    // literal, so the call on line 7 logs with a tag over the limit. Go looks
    // the name up only in the calling file and misses it. The short tag on
    // line 8 reports in neither.
    @Test fun tagConstantInAnotherFile() {
        val sources = mapOf(
            "Tags.kt" to """
                package demo

                object Tags {
                    const val LONG = "TagDeclaredInAnotherFileXXX"
                    const val SHORT = "Short"
                }
            """.trimIndent(),
            "User.kt" to """
                package demo

                import android.util.Log

                class User {
                    fun log() {
                        Log.d(Tags.LONG, "m")
                        Log.d(Tags.SHORT, "m")
                    }
                }
            """.trimIndent(),
        )
        assertEquals(listOf("User.kt" to 7), findings(sources))
    }
}
