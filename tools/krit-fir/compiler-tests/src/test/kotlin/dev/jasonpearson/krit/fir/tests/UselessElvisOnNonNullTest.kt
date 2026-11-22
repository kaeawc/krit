package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import kotlin.test.Test
import kotlin.test.assertEquals

class UselessElvisOnNonNullTest {
    @Test
    fun keepsGoMessageAndLeftOperandAnchor() {
        val source = """
            fun anchor(value: String): String {
                return (value) ?:
                    "fallback"
            }
        """.trimIndent()
        val found = KritFirProbe.diagnose(
            mapOf("Anchor.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UselessElvisOnNonNull")),
        ).filter { it.file == "Anchor.kt" }
        assertEquals(1, found.size)
        assertEquals(2, found.single().line)
        assertEquals("UselessElvisOnNonNull", found.single().name)
        assertEquals(
            "[UselessElvisOnNonNull] Useless elvis (?:) on non-nullable '(value)'. The fallback is dead code.",
            found.single().message,
        )
    }
}
