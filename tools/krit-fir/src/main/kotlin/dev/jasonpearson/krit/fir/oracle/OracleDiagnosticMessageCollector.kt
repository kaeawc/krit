package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import java.io.File

/**
 * `MessageCollector` that filters K2 warnings down to the same factory
 * subset krit-types retains (`UNREACHABLE_CODE`, `USELESS_ELVIS`,
 * `CAST_NEVER_SUCCEEDS`) and routes them through an [OracleCollector]
 * as [DiagnosticPayload]s.
 *
 * K2 doesn't expose factory names through the `MessageCollector`
 * surface — only severity, message text, and location. We recover the
 * factory by matching against the compiler's published message
 * templates (`FirErrorsDefaultMessages`). Any warning that doesn't
 * match a retained factory is dropped; messages emitted by the
 * krit-fir plugin's own checkers (identified by the `[RULE_NAME]`
 * prefix from `KritDiagnosticsRendering`) are also skipped so plugin
 * findings don't bleed into the oracle's diagnostic projection.
 */
internal class OracleDiagnosticMessageCollector(
    private val collector: OracleCollector,
) : MessageCollector {

    override fun clear() {}
    override fun hasErrors(): Boolean = false

    override fun report(
        severity: CompilerMessageSeverity,
        message: String,
        location: CompilerMessageSourceLocation?,
    ) {
        if (severity !in retainedSeverities) return
        if (location == null) return
        if (pluginDiagnosticPrefix.containsMatchIn(message)) return

        val factory = factoryFor(message) ?: return

        val canonicalPath = try {
            File(location.path).canonicalPath
        } catch (_: Exception) {
            location.path
        }
        val offsets = collector.offsetsFor(canonicalPath) ?: collector.offsetsFor(location.path)
        val startByte: Int
        val endByte: Int
        if (offsets != null && location.lineEnd > 0 && location.columnEnd > 0) {
            val startCharOffset = offsets.charOffsetFor(location.line, location.column)
            val endCharOffset = offsets.charOffsetFor(location.lineEnd, location.columnEnd)
            startByte = offsets.byteOffsetAt(startCharOffset)
            endByte = offsets.byteOffsetAt(endCharOffset)
        } else {
            startByte = 0
            endByte = 0
        }

        collector.addDiagnostic(
            canonicalPath,
            DiagnosticPayload(
                factoryName = factory,
                severity = severity.wireString(),
                message = message,
                line = location.line,
                col = location.column,
                startByte = startByte,
                endByte = endByte,
            ),
        )
    }

    private fun factoryFor(message: String): String? {
        for ((prefix, factory) in factoryPrefixes) {
            if (message.startsWith(prefix)) return factory
        }
        return null
    }

    private fun CompilerMessageSeverity.wireString(): String = when (this) {
        CompilerMessageSeverity.ERROR -> "ERROR"
        CompilerMessageSeverity.STRONG_WARNING -> "WARNING"
        CompilerMessageSeverity.WARNING -> "WARNING"
        else -> name
    }

    companion object {
        private val retainedSeverities = setOf(
            CompilerMessageSeverity.WARNING,
            CompilerMessageSeverity.STRONG_WARNING,
        )

        // FirErrorsDefaultMessages templates as of Kotlin 2.3.21. The
        // entries below match the literal message-text prefixes K2 emits
        // through the CLI MessageCollector — see the message templates in
        // `org.jetbrains.kotlin.fir.analysis.diagnostics.FirErrorsDefaultMessages`.
        // `USELESS_ELVIS_LEFT_IS_NULL` / `USELESS_ELVIS_RIGHT_IS_NULL` are
        // separate factories that krit-types intentionally does NOT retain;
        // matching only the "always returns the left operand" template
        // keeps parity with the krit-types projection.
        private val factoryPrefixes: List<Pair<String, String>> = listOf(
            "Elvis operator (?:) always returns" to "USELESS_ELVIS",
            "This cast can never succeed" to "CAST_NEVER_SUCCEEDS",
            "Unreachable code" to "UNREACHABLE_CODE",
        )

        private val pluginDiagnosticPrefix = Regex("""\[[A-Z_]+]""")
    }
}

/**
 * Convert a 1-based (line, column) pair into the 0-based char offset
 * the rest of the offset table operates on. Clamps out-of-range
 * positions so a malformed `CompilerMessageSourceLocation` doesn't
 * crash the diagnostic projection.
 */
internal fun FileOffsetTable.charOffsetFor(line: Int, column: Int): Int {
    val safeLine = (line - 1).coerceAtLeast(0)
    val safeCol = (column - 1).coerceAtLeast(0)
    return lineStartOffset(safeLine) + safeCol
}
