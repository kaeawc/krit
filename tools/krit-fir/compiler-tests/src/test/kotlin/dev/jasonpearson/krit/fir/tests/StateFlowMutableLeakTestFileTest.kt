package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The Go rule skips scanner.IsTestFile files. The checker must take that
// classification from the check request (FirRule.isTestFile), never from its
// own reading of the path: a file the request lists is skipped wherever it
// lives, and a file it does not list is checked even under src/test.
class StateFlowMutableLeakTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        import kotlinx.coroutines.flow.MutableStateFlow

        class ViewModel {
            val state = MutableStateFlow(0)
        }
    """.trimIndent()

    private val sources = mapOf(
        "Listed.kt" to source("listed"),
        "src/test/kotlin/Unlisted.kt" to source("unlisted"),
    )

    private fun leaks(testFiles: Set<String>): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("StateFlowMutableLeak")),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "StateFlowMutableLeak" }.groupingBy { it.file }.eachCount()
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to 1), leaks(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirPath() {
        assertEquals(mapOf("Listed.kt" to 1, "Unlisted.kt" to 1), leaks(emptySet()))
    }

    @Test fun everyListedFileIsSkipped() {
        assertEquals(emptyMap(), leaks(sources.keys))
    }
}
