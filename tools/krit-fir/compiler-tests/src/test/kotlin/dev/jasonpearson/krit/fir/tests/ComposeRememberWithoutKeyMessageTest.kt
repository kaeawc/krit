package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The key list in ComposeRememberWithoutKey's message matches Go's: it names locals derived from parameters (as
// "values") and leaves out a parameter the function writes back into the remembered object.
class ComposeRememberWithoutKeyMessageTest {

    private val source = """
        package remembermessage

        import androidx.compose.runtime.Composable
        import androidx.compose.runtime.remember

        fun use(x: Any?) {}

        class DragState(val scope: Any, var index: Int)

        @Composable
        fun Derived(data: List<Int>, scale: Int) {
            val total = data.size
            val v = remember { total * scale }
            use(v)
        }

        @Composable
        fun Resynced(scope: Any, index: Int) {
            val target = remember { DragState(scope, index) }.also { it.index = index }
            use(target)
        }
    """.trimIndent()

    private fun messages(): List<String> {
        val result = KritFirProbe.compile(
            mapOf("M.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("ComposeRememberWithoutKey")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.sortedBy { it.line }.map { it.message }
    }

    @Test fun keyListNamesDerivedLocalsAndDropsResyncedParameters() = assertEquals(
        listOf(
            "[ComposeRememberWithoutKey] remember { } reads enclosing values total, scale but has no keys; the cached " +
                "value won't update when they change. Pass remember(total, scale) { ... }.",
            "[ComposeRememberWithoutKey] remember { } reads enclosing parameter scope but has no keys; the cached " +
                "value won't update when scope changes. Pass remember(scope) { ... }.",
        ),
        messages(),
    )
}
