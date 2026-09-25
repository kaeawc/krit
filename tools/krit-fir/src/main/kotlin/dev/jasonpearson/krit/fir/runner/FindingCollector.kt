package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import java.io.File

data class Finding(
    val path: String,
    val line: Int,
    val col: Int,
    val startByte: Int = 0,
    val endByte: Int = 0,
    val rule: String,
    val severity: String,
    val message: String,
    val confidence: Double = 1.0,
)

// MessageCollector implementation that captures krit diagnostics (identified by the
// [RULE_NAME] prefix set in KritDiagnosticsRendering), filters to the requested files,
// and optionally restricts to the enabled rule set (empty = all rules).
//
// It also records which requested files the compiler could not analyze cleanly, so Go
// only treats the checker verdict as authoritative where the compilation was sound:
//  - [errorFiles]: requested files with an ERROR-severity compiler diagnostic (an
//    unresolved reference from a missing classpath entry, a syntax error, ...), mapped
//    to the first such message.
//  - [globalErrors]: ERROR diagnostics with no source location. They describe the
//    compilation as a whole, so every requested file is gated by them.
//  - [exceptions]: compiler crashes (EXCEPTION severity).
class FindingCollector(
    private val requestedPaths: Map<String, String>,
    private val enabledRules: Set<String> = emptySet(),
) : MessageCollector {
    val findings = mutableListOf<Finding>()
    val errorFiles = linkedMapOf<String, String>()
    val globalErrors = mutableListOf<String>()
    val exceptions = mutableListOf<String>()

    private var _hasErrors = false

    override fun clear() {}
    override fun hasErrors() = _hasErrors

    override fun report(
        severity: CompilerMessageSeverity,
        message: String,
        location: CompilerMessageSourceLocation?,
    ) {
        if (severity == CompilerMessageSeverity.EXCEPTION) {
            _hasErrors = true
            exceptions += message
            return
        }
        if (severity == CompilerMessageSeverity.ERROR && !pluginDiagnosticRe.containsMatchIn(message)) {
            _hasErrors = true
            if (location == null) {
                globalErrors += message
            } else {
                requestedPaths[canonical(location.path)]?.let { errorFiles.putIfAbsent(it, message) }
            }
        }

        if (severity !in reportable) return
        if (location == null) return

        // Only record findings for the files the caller asked to check.
        val requestedPath = requestedPaths[canonical(location.path)] ?: return

        // Only count diagnostics emitted by our plugin (identified by [RULE_NAME] prefix).
        val match = pluginDiagnosticRe.find(message) ?: return
        val ruleName = match.groupValues[1]
        if (enabledRules.isNotEmpty() && ruleName !in enabledRules) return
        val msg = message.substringAfter("] ").trim()

        findings.add(
            Finding(
                path = requestedPath,
                line = location.line,
                col = location.column,
                rule = ruleName,
                severity = if (severity == CompilerMessageSeverity.ERROR) "error" else "warning",
                message = msg,
                confidence = 1.0,
            )
        )
    }

    private fun canonical(path: String): String =
        try { File(path).canonicalPath } catch (_: Exception) { path }

    companion object {
        private val reportable = setOf(
            CompilerMessageSeverity.WARNING,
            CompilerMessageSeverity.STRONG_WARNING,
            CompilerMessageSeverity.ERROR,
        )
        private val pluginDiagnosticRe = Regex("""^\[([A-Za-z][A-Za-z0-9_]*)]""")
    }
}
