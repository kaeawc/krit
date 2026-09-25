package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The Go rule skips scanner.IsTestFile files. The checker takes that
// classification from the check request, never from the path.
class JavaObjectInputStreamTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        import java.io.InputStream
        import java.io.ObjectInputStream

        class Decoder {
            fun open(input: InputStream): ObjectInputStream = ObjectInputStream(input)
        }
    """.trimIndent()

    private val sources = mapOf(
        "Listed.kt" to source("listed"),
        "src/test/kotlin/Unlisted.kt" to source("unlisted"),
    )

    private fun findings(testFiles: Set<String>): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("JavaObjectInputStream")),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "JavaObjectInputStream" }.groupingBy { it.file }.eachCount()
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to 1), findings(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirPath() {
        assertEquals(mapOf("Listed.kt" to 1, "Unlisted.kt" to 1), findings(emptySet()))
    }
}
