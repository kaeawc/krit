package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// Lint does not analyze test sources, and Go's Wakelock and WrongCall skip scanner.IsTestFile files. A test that
// verifies `acquire()` on a mock, or calls a View callback on purpose, reports nothing.
class WakelockWrongCallTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        import android.content.Context
        import android.graphics.Canvas
        import android.os.PowerManager
        import android.view.View

        class Holder(private val powerManager: PowerManager) {
            fun run() {
                val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "a:b")
                lock.acquire()
            }
        }

        class Dial(context: Context) : View(context) {
            fun redraw(canvas: Canvas) {
                onDraw(canvas)
            }
        }
    """.trimIndent()

    private val rules = setOf("Wakelock", "WrongCall")

    private val sources = mapOf(
        "Listed.kt" to source("listed"),
        "Unlisted.kt" to source("unlisted"),
    )

    private fun findings(testFiles: Set<String>): Map<String, List<String>> {
        val result = KritFirProbe.compile(sources, FirRuleCompileContext(enabledRuleIds = rules), testFiles = testFiles)
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name in rules }.groupBy({ it.file }, { it.name }).mapValues { it.value.sorted() }
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to listOf("Wakelock", "WrongCall")), findings(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreChecked() {
        assertEquals(
            mapOf("Listed.kt" to listOf("Wakelock", "WrongCall"), "Unlisted.kt" to listOf("Wakelock", "WrongCall")),
            findings(emptySet()),
        )
    }
}
