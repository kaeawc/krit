package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class UnnecessarySafeCallMessagesTest {
    @Test
    fun preservesGoMessagesAndOperatorLines() {
        val source = """
            class SafeCallMessageHolder(val text: String)
            fun messages(parameter: String, holder: SafeCallMessageHolder) {
                val local: String = "text"
                println(local?.length)
                println(parameter
                    ?.length)
                println(holder.text?.length)
            }
        """.trimIndent()
        val diagnostics = KritFirProbe.diagnose(
            mapOf("Messages.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnnecessarySafeCall")),
        ).filter { it.file == "Messages.kt" }
        assertEquals(listOf(4, 6, 7), diagnostics.map { it.line })
        assertEquals(
            listOf(
                "Unnecessary safe call (?.) on non-nullable 'local'.",
                "Unnecessary safe call (?.) on non-nullable parameter 'parameter'.",
                "Unnecessary safe call (?.): 'holder.text' is already non-null here (non-null type or smart cast).",
            ),
            diagnostics.map { it.message.removePrefix("[UnnecessarySafeCall] ") },
        )
    }
}
