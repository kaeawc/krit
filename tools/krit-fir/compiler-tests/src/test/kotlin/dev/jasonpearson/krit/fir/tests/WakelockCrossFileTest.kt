package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// Wakelock decided that a property was a local of the function under check by
// comparing source offsets alone, never the file. A top-level `val lock` in
// another file whose offsets happen to fall inside the function's range was
// taken for a local `newWakeLock()` and `lock.acquire()` was reported, a false
// positive that depended on where the declaration sat in its own file.
class WakelockCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("Wakelock")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "Wakelock" }.map { it.file to it.line }
    }

    // B.kt's `lock` is declared at offsets inside f() in A.kt (padded by the
    // comment lines), but is a top-level property of another file.
    @Test fun topLevelValInAnotherFileWithCoincidingOffsetsIsNotLocal() {
        val filler = "    // " + "x".repeat(60)
        val sources = mapOf(
            "B.kt" to """
                package demo

                class Locker { fun newWakeLock(): Locker = this; fun acquire() {}; fun release() {} }
                val lock = Locker().newWakeLock()
            """.trimIndent(),
            "A.kt" to "package demo\n\nfun f() {\n$filler\n$filler\n$filler\n$filler\n    lock.acquire()\n$filler\n$filler\n}\n",
        )
        assertEquals(emptyList(), findings(sources))
    }
}
