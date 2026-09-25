package dev.jasonpearson.krit.fir.checkers.i18n

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.toKtLightSourceElement
import org.jetbrains.kotlin.util.getChildren

// Flags the Kotlin 1.5+ stdlib case conversions `uppercase()` / `lowercase()`
// (kotlin.text, on String and Char) called without a Locale argument.
//
// Mirrors the Go rule's evidence on top of FIR resolution:
// - the call resolves to kotlin.text.uppercase / kotlin.text.lowercase with no
//   arguments (the `(locale: Locale)` overloads are never flagged);
// - `.gradle.kts` scripts are skipped;
// - a call whose explicit receiver's source text contains one of Go's
//   ASCII-invariant identifiers (`currencyCode`, `url`, `mimeType`, `hex`,
//   ...) is skipped. The match is a plain substring match on the receiver
//   text, exactly as Go does it, so `ghostName` is skipped because it contains
//   `host`;
// - the finding is reported on the line where the whole qualified call starts
//   (the receiver's first line), as Go reports on its call_expression.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go matches any `<receiver>.uppercase()` / `.lowercase()` by
//   name. A project member or extension with that name (a class member, a
//   member or local extension, a same-package `String.uppercase()` or one
//   brought in by an explicit or star import, another function imported
//   `as uppercase`, or a local function-typed value invoked as `s.uppercase()`)
//   is not a stdlib case conversion and takes no Locale, so FIR does not
//   report it.
// - Recall: Go only sees calls with an explicit receiver and spelled
//   `uppercase` / `lowercase`. FIR also reports the stdlib calls made on an
//   implicit receiver (`with(s) { uppercase() }`, an extension body), through
//   an import alias (`import kotlin.text.uppercase as up`) and spelled with
//   backticks (`s.`uppercase`()`). An implicit-receiver call has no receiver
//   text, so the ASCII-invariant exemption cannot apply to it:
//   `with(currencyCode) { uppercase() }` is reported.
internal object UpperLowerInvariantMisuse : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UpperLowerInvariantMisuse"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UpperLowerInvariantMisuse)
    }

    private val kotlinText = FqName("kotlin.text")
    private val methodNames = setOf("uppercase", "lowercase")

    // Go's containsASCIIInvariantIdentifier list, matched as substrings of
    // the receiver's source text.
    private val asciiInvariantIdentifiers = listOf(
        "currencyCode", "currency", "isoCode", "countryCode", "languageCode",
        "iban", "IBAN",
        "mimeType", "contentType", "MIME",
        "protocol", "scheme", "host", "uri", "URI", "url", "URL",
        "uuid", "UUID", "guid", "GUID",
        "serviceId", "deviceId",
        "cipher", "algorithm", "digest",
        "columnName", "columnNames", "tableName", "indexName",
        "hex", "Hex", "toHexString", "toHex",
        "verb", "httpMethod", "method", "requestMethod",
    )

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId.packageName != kotlinText || callableId.className != null) return
        val name = callableId.callableName.asString()
        if (name !in methodNames) return
        if (expression.argumentList.arguments.isNotEmpty()) return
        if (context.containingFile?.name?.endsWith(".gradle.kts") == true) return

        val source = expression.source ?: return
        val qualified = qualifiedCall(source)
        if (qualified != null) {
            val receiver = significantChildren(qualified, source.treeStructure).firstOrNull() ?: return
            val receiverText = source.treeStructure.toString(receiver).toString()
            if (asciiInvariantIdentifiers.any { receiverText.contains(it) }) return
        }
        report(qualified?.let { sourceOf(it, source) } ?: source, message(name))
    }

    private fun message(name: String) =
        "'$name()' called without explicit Locale. Pass 'Locale.ROOT' for case-insensitive comparison or use a display-locale variant for user-facing text."

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call, or null when the call has no explicit receiver. K2 gives a dot call
    // the whole qualified expression as its source; a safe call keeps the
    // selector call expression, so step up to its parent.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val tree = source.treeStructure
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = tree.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = significantChildren(parent, tree)
        return parent.takeIf { parts.size > 1 && parts.last() == node }
    }

    // A source element for [node], a node in [anchor]'s tree, keeping the
    // anchor's offset shift between tree offsets and file offsets.
    private fun sourceOf(node: LighterASTNode, anchor: KtSourceElement): KtSourceElement {
        if (node == anchor.lighterASTNode) return anchor
        val shift = anchor.startOffset - anchor.lighterASTNode.startOffset
        return node.toKtLightSourceElement(
            anchor.treeStructure,
            startOffset = node.startOffset + shift,
            endOffset = node.endOffset + shift,
        )
    }

    private fun significantChildren(
        node: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): List<LighterASTNode> =
        node.getChildren(tree).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS
        }
}
