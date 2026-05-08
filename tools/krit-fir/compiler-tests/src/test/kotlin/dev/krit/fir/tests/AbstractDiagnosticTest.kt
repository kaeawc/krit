package dev.krit.fir.tests

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File
import kotlin.test.fail

// Base class for golden-value FIR diagnostic tests.
//
// Test data files live under src/test/data/diagnostic/ and use inline marker syntax:
//   expr.<!DIAGNOSTIC_NAME!>token<!>
//
// Shared stubs (for coroutines, Compose, etc.) live in src/test/data/stubs/ and
// are compiled alongside every test file so that FQ names resolve correctly.
//
// Running `./gradlew :compiler-tests:generateTests` regenerates the test class;
// `./gradlew :compiler-tests:test` runs the suite.
abstract class AbstractDiagnosticTest {

    private val dataDir: File
        get() = File("src/test/data/diagnostic")

    private val stubsDir: File
        get() = File("src/test/data/stubs")

    fun runDiagnosticTest(relativePath: String) {
        val file = dataDir.resolve(relativePath)
        require(file.exists()) { "Test data file not found: ${file.absolutePath}" }

        val raw = file.readText()
        val (cleanSource, expected) = parseMarkers(raw)

        val actual = collectDiagnosticsOnLines(cleanSource, file.name)

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

    // Compiles `source` together with shared stubs using the krit-fir plugin, and
    // returns line numbers on which the plugin emitted a diagnostic.
    // Only messages prefixed with [DIAGNOSTIC_NAME] (from KritDiagnosticsRendering)
    // are counted; standard Kotlin compiler warnings are ignored.
    private fun collectDiagnosticsOnLines(source: String, filename: String): List<Int> {
        val pluginJar = requireNotNull(locatePluginJar()) {
            "krit-fir plugin JAR not found. " +
                "Set system property 'krit.fir.plugin.jar' or run `./gradlew :jar` first."
        }

        val tmpDir = kotlin.io.path.createTempDirectory("krit-fir-test").toFile()
        try {
            tmpDir.resolve(filename).writeText(source)

            // Copy shared stubs so that library FQ names (kotlinx.coroutines.flow, etc.) resolve.
            if (stubsDir.isDirectory) {
                stubsDir.listFiles { f -> f.extension == "kt" }?.forEach { stub ->
                    stub.copyTo(tmpDir.resolve(stub.name), overwrite = true)
                }
            }

            val outDir = tmpDir.resolve("out").apply { mkdirs() }
            val lines = mutableListOf<Int>()
            val collector = object : MessageCollector {
                override fun clear() {}
                override fun hasErrors() = false
                override fun report(
                    severity: CompilerMessageSeverity,
                    message: String,
                    location: CompilerMessageSourceLocation?,
                ) {
                    // Only count diagnostics emitted by our plugin (identified by the
                    // [DIAGNOSTIC_NAME] prefix set in KritDiagnosticsRendering).
                    if (location != null && severity in reportable && pluginDiagnosticRe.containsMatchIn(message)) {
                        lines.add(location.line)
                    }
                }
            }

            // Locate kotlin-stdlib.jar so the embedded compiler can resolve built-in
            // declarations (kotlin.Any, println, etc.) in the test sources.
            val stdlibJar = System.getProperty("kotlin.stdlib.jar")?.let { File(it).takeIf { f -> f.exists() } }

            K2JVMCompiler().exec(
                collector,
                Services.EMPTY,
                K2JVMCompilerArguments().apply {
                    freeArgs = listOf(tmpDir.absolutePath)
                    destination = outDir.absolutePath
                    noStdlib = true
                    noReflect = true
                    if (stdlibJar != null) {
                        classpath = stdlibJar.absolutePath
                    }
                    pluginClasspaths = arrayOf(pluginJar.absolutePath)
                },
            )
            return lines
        } finally {
            tmpDir.deleteRecursively()
        }
    }

    private fun locatePluginJar(): File? {
        val fromSysProp = System.getProperty("krit.fir.plugin.jar")
        if (fromSysProp != null) return File(fromSysProp).takeIf { it.exists() }
        return File("../build/libs")
            .takeIf { it.isDirectory }
            ?.listFiles { f -> f.name.startsWith("krit-fir") && f.name.endsWith(".jar") }
            ?.firstOrNull()
    }

    data class ExpectedDiagnostic(val line: Int, val name: String)

    companion object {
        private val reportable = setOf(
            CompilerMessageSeverity.WARNING,
            CompilerMessageSeverity.STRONG_WARNING,
            CompilerMessageSeverity.ERROR,
        )
        // Matches the [DIAGNOSTIC_NAME] prefix set in KritDiagnosticsRendering.
        private val pluginDiagnosticRe = Regex("""\[[A-Z_]+]""")
    }
}
