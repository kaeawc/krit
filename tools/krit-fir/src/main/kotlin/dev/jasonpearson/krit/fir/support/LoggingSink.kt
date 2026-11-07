package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.supertypeNames
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol

private val SINK_SUPERTYPES = setOf("Tree", "DebugTree", "Handler", "Appender")
private val STDOUT_WORDS = listOf("stdout", "console", "standard output", "system.out", "system.err", "prints to")

/** Path markers of CLI tooling, scripts and test-support sources, where console output is intended. */
fun isPrintToolingPath(path: String?): Boolean {
    if (path == null) return false
    val slash = "/" + path.replace('\\', '/').trimStart('/')
    return slash.endsWith(".kts") || listOf("/tooling/", "/bazel/", "/scripts/", "/test-fixtures/", "/test-support/", "/test-utils/")
        .any { it in slash }
}

/**
 * Whether the checked call sits in a logging sink, as the Go print rules see it: inside a `log` override, in a
 * function whose KDoc says it writes to stdout or that is named `print` / `printXxx`, or in a class or object
 * named `*Logger` or extending a `Tree`, `DebugTree`, `Handler` or `Appender`.
 */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
fun insideLoggingSink(): Boolean {
    val function = context.containingDeclarations.lastOrNull { it is FirNamedFunctionSymbol } as? FirNamedFunctionSymbol
    if (function != null) {
        if (function.name.asString() == "log" && function.rawStatus.isOverride) return true
        if (documentedAsStdout(context, function.source)) return true
        if (namedForPrinting(function.name.asString())) return true
    }
    return context.containingDeclarations.any { symbol ->
        if (symbol !is FirClassSymbol<*>) return@any false
        if (symbol.classId.shortClassName.asString().endsWith("Logger")) return@any true
        val names = HashSet<String>()
        for (ref in symbol.fir.superTypeRefs) supertypeNames(ref, names)
        names.any { it in SINK_SUPERTYPES }
    }
}

// True for `print` and `printXxx`: a function whose purpose is console output.
private fun namedForPrinting(name: String): Boolean =
    name.startsWith("print") && (name.length == 5 || name[5].isUpperCase())

// True when the KDoc of the declaration at [source] says it writes to the console. The declaration's source can
// start at its own KDoc, so that is checked first, then the comment directly before it.
private fun documentedAsStdout(context: CheckerContext, source: KtSourceElement?): Boolean {
    source ?: return false
    val text = context.containingFile?.getContentsAsStream()?.readBytes()?.decodeToString() ?: return false
    val start = source.startOffset.coerceIn(0, text.length)
    val own = text.substring(start).trimStart()
    if (own.startsWith("/**")) {
        val end = own.indexOf("*/")
        if (end >= 0) return mentionsStdout(own.substring(0, end))
    }
    val before = text.substring(0, start).trimEnd()
    if (!before.endsWith("*/")) return false
    val doc = before.substring(before.lastIndexOf("/**").takeIf { it >= 0 } ?: return false)
    return mentionsStdout(doc)
}

private fun mentionsStdout(doc: String): Boolean = STDOUT_WORDS.any { it in doc.lowercase() }
