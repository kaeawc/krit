package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

// android.util.Log is a Java class with only static members, so Kotlin cannot
// use `Log` as a value: `with(Log) { d(...) }`, `Log.run { e(...) }`, and a
// `Log.run { isLoggable(...) }` guard do not compile. A file that does not
// compile is not authoritative for FIR, so Go's verdict stands on these
// shapes and they are not divergences.
class LogConditionalReceiverShapesTest {

    private fun compile(body: String) = KritFirProbe.compile(
        mapOf(
            "Shape.kt" to """
                package shape

                import android.util.Log

                fun shape() {
                    $body
                }
            """.trimIndent(),
        ),
        FirRuleCompileContext(enabledRuleIds = setOf("LogConditional")),
    )

    @Test fun withLogDoesNotCompile() {
        assertFalse(compile("""with(Log) { d("Tag", "m") }""").clean)
    }

    @Test fun logRunDoesNotCompile() {
        assertFalse(compile("""Log.run { e("Tag", "m") }""").clean)
    }

    @Test fun logRunIsLoggableGuardDoesNotCompile() {
        assertFalse(compile("""if (Log.run { isLoggable("Tag", Log.DEBUG) }) Log.d("Tag", "m")""").clean)
    }

    // The qualified spellings compile, so the shapes above fail on the
    // receiver, not on the snippet.
    @Test fun qualifiedCallsCompile() {
        val result = compile("""if (Log.isLoggable("Tag", Log.DEBUG)) Log.d("Tag", "m")""")
        assertTrue(result.clean, result.problems())
    }
}
