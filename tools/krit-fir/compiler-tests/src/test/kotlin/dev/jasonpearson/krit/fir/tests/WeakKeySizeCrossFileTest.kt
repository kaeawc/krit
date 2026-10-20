package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// WeakKeySize cases a single-file golden cannot express: a generator val
// declared in another file.
class WeakKeySizeCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("WeakKeySize")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "WeakKeySize" }.map { it.file to it.line }
    }

    // Each val is declared in Keys.kt with the AES generator as its only value, so every 64-bit call in Use.kt
    // would be weak if the rule read the generator's initializer: an object member brought in by import (line 7),
    // a top-level val (line 8), and a protected member val inherited from a base class (line 13). A rule that
    // follows an initializer of another file gives a different verdict on an incremental slice, where that file
    // is a class file without initializers (#873), so none of the three is reported. Go searches only the
    // calling function for the generator's getInstance and misses all three as well. The same vals in the file
    // that uses them are reported (generatorValInTheSameFile).
    @Test fun generatorValInAnotherFile() {
        val sources = mapOf(
            "Keys.kt" to """
                package q1

                import javax.crypto.KeyGenerator

                object Keys {
                    val shared: KeyGenerator = KeyGenerator.getInstance("AES")
                }

                val topShared: KeyGenerator = KeyGenerator.getInstance("AES")

                open class BaseKeys {
                    protected val baseGen: KeyGenerator = KeyGenerator.getInstance("AES")
                }
            """.trimIndent(),
            "Use.kt" to """
                package q1

                import q1.Keys.shared

                fun useShared() {
                    val unused = 0
                    shared.init(64)
                    topShared.init(64)
                }

                class UsesBase : BaseKeys() {
                    fun weak() {
                        baseGen.init(64)
                        baseGen.init(128)
                    }
                }
            """.trimIndent(),
        )
        assertEquals(emptyList(), findings(sources))
    }

    // The same three vals in the file that uses them: all three 64-bit calls are weak, the 128-bit one is not.
    @Test fun generatorValInTheSameFile() {
        val sources = mapOf(
            "Use.kt" to """
                package q1

                import javax.crypto.KeyGenerator
                import q1.Keys.shared

                object Keys {
                    val shared: KeyGenerator = KeyGenerator.getInstance("AES")
                }

                val topShared: KeyGenerator = KeyGenerator.getInstance("AES")

                open class BaseKeys {
                    protected val baseGen: KeyGenerator = KeyGenerator.getInstance("AES")
                }

                fun useShared() {
                    shared.init(64)
                    topShared.init(64)
                }

                class UsesBase : BaseKeys() {
                    fun weak() {
                        baseGen.init(64)
                        baseGen.init(128)
                    }
                }
            """.trimIndent(),
        )
        assertEquals(listOf("Use.kt" to 17, "Use.kt" to 18, "Use.kt" to 23), findings(sources).sortedBy { it.second })
    }
}
