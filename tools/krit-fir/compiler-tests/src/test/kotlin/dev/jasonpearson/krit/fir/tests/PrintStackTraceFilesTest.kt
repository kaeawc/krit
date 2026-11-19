package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// PrintStackTrace path exemptions. Go and detekt report a caught exception's
// printStackTrace() and Thread.dumpStack() in every source file, test sources
// and tooling paths included. Only the receivers Go does not report (a
// parameter here) skip those files.
class PrintStackTraceFilesTest {

    private fun findings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("PrintStackTrace")),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "PrintStackTrace" }.groupingBy { it.file }.eachCount()
    }

    // Three findings Go reports (caught exception, caught exception with a
    // stream, Thread.dumpStack()) and one it does not (a parameter).
    private fun printer(pkg: String) = """
        package $pkg

        fun work() {}

        fun caught() {
            try {
                work()
            } catch (e: Exception) {
                e.printStackTrace()
                e.printStackTrace(System.err)
            }
        }

        fun dump() {
            Thread.dumpStack()
        }

        fun parameter(error: Throwable) {
            error.printStackTrace()
        }
    """.trimIndent()

    @Test fun productionFileReportsEveryShape() {
        assertEquals(mapOf("Main.kt" to 4), findings(mapOf("Main.kt" to printer("main"))))
    }

    @Test fun testFileKeepsTheShapesGoReports() {
        val sources = mapOf("Listed.kt" to printer("listed"), "Unlisted.kt" to printer("unlisted"))
        assertEquals(
            mapOf("Listed.kt" to 3, "Unlisted.kt" to 4),
            findings(sources, testFiles = setOf("Listed.kt")),
        )
    }

    @Test fun toolingPathKeepsTheShapesGoReports() {
        val sources = mapOf(
            "app/src/main/kotlin/App.kt" to printer("app"),
            "tooling/cli/src/main/kotlin/Cli.kt" to printer("cli"),
        )
        val relative = sources.keys.associateWith { it }
        assertEquals(mapOf("App.kt" to 4, "Cli.kt" to 3), findings(sources, scanPaths = relative))
    }
}
