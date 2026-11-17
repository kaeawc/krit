package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The Go Android Lint rules skip scanner.IsTestFile files. WakelockTimeout, SetWorldReadable and SetWorldWritable take
// that classification from the check request, so a test that verifies `acquire()` on a mock reports nothing.
class AndroidLintTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        import android.os.PowerManager
        import java.io.File

        class Holder(private val lock: PowerManager.WakeLock, private val file: File) {
            fun run() {
                lock.acquire()
                file.setReadable(true, false)
                file.setWritable(true, false)
            }
        }
    """.trimIndent()

    private val rules = setOf("WakelockTimeout", "SetWorldReadable", "SetWorldWritable")

    private val sources = mapOf(
        "Listed.kt" to source("listed"),
        "src/test/kotlin/Unlisted.kt" to source("unlisted"),
    )

    private fun findings(testFiles: Set<String>): Map<String, Int> {
        val result = KritFirProbe.compile(sources, FirRuleCompileContext(enabledRuleIds = rules), testFiles = testFiles)
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name in rules }.groupingBy { it.file }.eachCount()
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to 3), findings(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirPath() {
        assertEquals(mapOf("Listed.kt" to 3, "Unlisted.kt" to 3), findings(emptySet()))
    }

    @Test fun everyListedFileIsSkipped() {
        assertEquals(emptyMap(), findings(sources.keys))
    }
}
