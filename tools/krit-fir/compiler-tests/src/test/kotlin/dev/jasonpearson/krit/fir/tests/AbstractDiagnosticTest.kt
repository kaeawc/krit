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

        val actual = KritFirProbe.diagnose(mapOf(file.name to cleanSource))
            .filter { it.file == file.name }
            .map { it.line }

        val expectedLines = expected.map { it.line }.toSet()
        val missingLines = expectedLines - actual.toSet()
        val unexpectedLines = actual.toSet() - expectedLines

        if (missingLines.isNotEmpty() || unexpectedLines.isNotEmpty()) {
            fail(buildString {
                appendLine("Diagnostic mismatch in $relativePath")
                if (missingLines.isNotEmpty()) {
                    appendLine("  Expected diagnostics on lines (not found): $missingLines")
                    for (d in expected.filter { it.line in missingLines }) {
                        appendLine("    line ${d.line}: ${d.name}")
                    }
                }
                if (unexpectedLines.isNotEmpty()) {
                    appendLine("  Unexpected krit diagnostics on lines: $unexpectedLines")
                }
            })
        }
    }

    // Returns (cleanSource, expectedDiagnostics).
    // Strips <!DIAG_NAME!>token<!> markers and records expected (line, name) pairs.
    private fun parseMarkers(source: String): Pair<String, List<ExpectedDiagnostic>> {
        val markerRe = Regex("""<!([A-Z_]+)!>(.*?)<!>""", RegexOption.DOT_MATCHES_ALL)
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
