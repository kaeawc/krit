package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The Go rule skips scanner.IsTestFile files, because a test may call gc() to force finalization or to measure
// leaks. The checker takes that classification from the check request.
class ExplicitGarbageCollectionCallTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        class Holder {
            fun run() {
                System.gc()
                Runtime.getRuntime().runFinalization()
            }
        }
    """.trimIndent()

    private val rules = setOf("ExplicitGarbageCollectionCall")

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
        assertEquals(mapOf("Unlisted.kt" to 2), findings(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirPath() {
        assertEquals(mapOf("Listed.kt" to 2, "Unlisted.kt" to 2), findings(emptySet()))
    }

    @Test fun everyListedFileIsSkipped() {
        assertEquals(emptyMap(), findings(sources.keys))
    }
}
