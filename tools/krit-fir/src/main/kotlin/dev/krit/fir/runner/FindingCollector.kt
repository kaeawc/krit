package dev.krit.fir.runner

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
class FindingCollector(
    private val requestedPaths: Set<String>,
    private val enabledRules: Set<String> = emptySet(),
) : MessageCollector {
    val findings = mutableListOf<Finding>()
    val crashes = mutableMapOf<String, String>()

    private var _hasErrors = false

    override fun clear() {}
    override fun hasErrors() = _hasErrors

    override fun report(
        severity: CompilerMessageSeverity,
        message: String,
        location: CompilerMessageSourceLocation?,
    ) {
        if (severity == CompilerMessageSeverity.ERROR && !pluginDiagnosticRe.containsMatchIn(message)) {
            _hasErrors = true
            if (location != null) {
                val canonicalPath = try { File(location.path).canonicalPath } catch (_: Exception) { location.path }
                if (canonicalPath in requestedPaths || location.path in requestedPaths) {
                    crashes[location.path] = message
                }
            }
        }

        if (severity !in reportable) return
        if (location == null) return

        // Only record findings for the files the caller asked to check.
        val canonicalPath = try { File(location.path).canonicalPath } catch (_: Exception) { location.path }
        if (canonicalPath !in requestedPaths && location.path !in requestedPaths) return

        // Only count diagnostics emitted by our plugin (identified by [RULE_NAME] prefix).
        val match = pluginDiagnosticRe.find(message) ?: return
        val ruleName = match.groupValues[1]
        if (enabledRules.isNotEmpty() && ruleName !in enabledRules) return
        val msg = message.substringAfter("] ").trim()

        findings.add(
            Finding(
                path = location.path,
                line = location.line,
                col = location.column,
                rule = ruleName,
                severity = if (severity == CompilerMessageSeverity.ERROR) "error" else "warning",
                message = msg,
                confidence = 1.0,
            )
        )
    }

    companion object {
        private val reportable = setOf(
            CompilerMessageSeverity.WARNING,
            CompilerMessageSeverity.STRONG_WARNING,
            CompilerMessageSeverity.ERROR,
        )
        private val pluginDiagnosticRe = Regex("""\[([A-Z_]+)]""")
    }
}
