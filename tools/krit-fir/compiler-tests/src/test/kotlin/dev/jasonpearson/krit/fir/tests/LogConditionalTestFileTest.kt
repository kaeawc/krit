package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The Go rule skips scanner.IsTestFile files: test sources log without guards
// on purpose. The checker takes that classification from the check request,
// never from its own reading of the path.
class LogConditionalTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        import android.util.Log

        fun work() {
            Log.d("Tag", "unguarded")
        }
    """.trimIndent()

    private val sources = mapOf(
        "Listed.kt" to source("listed"),
        "src/test/kotlin/Unlisted.kt" to source("unlisted"),
    )

    private fun findings(testFiles: Set<String>): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("LogConditional")),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "LogConditional" }.groupingBy { it.file }.eachCount()
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to 1), findings(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirPath() {
        assertEquals(mapOf("Listed.kt" to 1, "Unlisted.kt" to 1), findings(emptySet()))
    }
}
