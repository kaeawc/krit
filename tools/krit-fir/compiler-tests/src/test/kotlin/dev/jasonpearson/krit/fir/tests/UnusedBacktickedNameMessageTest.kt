package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// A backticked name keeps its backticks in the message, as Go writes it. Krit keeps Go's message for a
// finding both engines report, so a finding only the checker reports must read the same way.
class UnusedBacktickedNameMessageTest {

    private val rules = setOf("UnusedPrivateProperty", "UnusedPrivateFunction", "UnusedParameter", "UnusedPrivateClass")

    // Each declaration is unused. Go keeps the property, function and class quiet because the name appears
    // elsewhere in the file; `on click` is called only as a member of Screen.
    private val source = """
        package unusedbacktickedname

        class Other {
            val `in` = 1
            fun `run it`() = 1
        }

        class Holder(`plain in`: Int) {
            private val `in` = 2
            private fun `run it`() = 3
            fun read(o: Other) = o.`in` + o.`run it`()
            fun take(`plain in`: Int) = `plain in`
        }

        private class `Name Clash`

        object Declares {
            val `Name Clash` = 1
        }

        fun clash() = Declares.`Name Clash`

        class Screen {
            fun `on click`() {}
        }

        fun memberCall(`on click`: () -> Unit, s: Screen) {
            s.`on click`()
        }
    """.trimIndent()

    private fun messages(): List<String> {
        val result = KritFirProbe.compile(mapOf("B.kt" to source), FirRuleCompileContext(enabledRuleIds = rules))
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name in rules }.sortedBy { it.line }.map { it.message.removePrefix("[${it.name}] ") }
    }

    @Test fun backticksStayInTheMessage() {
        assertEquals(
            listOf(
                "Private property ``plain in`` is unused.",
                "Private property ``in`` is unused.",
                "Private function ``run it`` is unused.",
                "Private class `Name Clash` is unused.",
                "Parameter '`on click`' is unused.",
            ),
            messages(),
        )
    }
}
