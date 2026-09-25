package dev.jasonpearson.krit.fir.tests

import java.io.File
import kotlin.test.fail

// Base class for golden-value FIR diagnostic tests.
//
// Test data files live under src/test/data/diagnostic/ and use inline marker syntax:
//   expr.<!DIAGNOSTIC_NAME!>token<!>
//
// Shared stubs (for coroutines, Compose, etc.) live in src/test/data/stubs/ and
// are compiled alongside every test file so that FQ names resolve correctly.
// The actual compile is delegated to [KritFirProbe], shared with the property tests.
//
// Running `./gradlew :compiler-tests:generateTests` regenerates the test class;
// `./gradlew :compiler-tests:test` runs the suite.
abstract class AbstractDiagnosticTest {

    private val dataDir: File
        get() = File("src/test/data/diagnostic")

    fun runDiagnosticTest(relativePath: String) {
        val file = dataDir.resolve(relativePath)
        require(file.exists()) { "Test data file not found: ${file.absolutePath}" }

        val raw = file.readText()
        val (cleanSource, expected) = parseMarkers(raw)

        // Compare (line, diagnostic-name) pairs, not just lines, so a marker that
        // expects rule A on a line where the checker actually emits rule B fails.
        val actual = KritFirProbe.diagnose(mapOf(file.name to cleanSource))
            .filter { it.file == file.name }
            .map { it.line to it.name }
            .toSet()
        val expectedSet = expected.map { it.line to it.name }.toSet()
        val missing = expectedSet - actual
        val unexpected = actual - expectedSet

        if (missing.isNotEmpty() || unexpected.isNotEmpty()) {
            fail(buildString {
                appendLine("Diagnostic mismatch in $relativePath")
                if (missing.isNotEmpty()) {
                    appendLine("  Expected (not found):")
                    for ((line, name) in missing.sortedBy { it.first }) appendLine("    line $line: $name")
                }
                if (unexpected.isNotEmpty()) {
                    appendLine("  Unexpected krit diagnostics:")
                    for ((line, name) in unexpected.sortedBy { it.first }) appendLine("    line $line: $name")
                }
            })
        }
    }

    // Returns (cleanSource, expectedDiagnostics).
    // Strips <!DIAG_NAME!>token<!> markers and records expected (line, name) pairs.
    private fun parseMarkers(source: String): Pair<String, List<ExpectedDiagnostic>> {
        val markerRe = Regex("""<!([A-Za-z][A-Za-z0-9_]*)!>(.*?)<!>""", RegexOption.DOT_MATCHES_ALL)
        val expected = mutableListOf<ExpectedDiagnostic>()
        val cleaned = source.lines().mapIndexed { idx, line ->
            var result = line
            for (match in markerRe.findAll(line)) {
                expected.add(ExpectedDiagnostic(line = idx + 1, name = match.groupValues[1]))
                result = result.replace(match.value, match.groupValues[2])
            }
            result
        }
        return Pair(cleaned.joinToString("\n"), expected)
    }

    data class ExpectedDiagnostic(val line: Int, val name: String)
}
