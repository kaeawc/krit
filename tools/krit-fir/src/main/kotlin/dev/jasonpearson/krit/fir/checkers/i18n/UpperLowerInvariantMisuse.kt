package dev.jasonpearson.krit.fir.checkers.i18n

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.text
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
//   `host`. A call on an implicit receiver gets the same exemption on the
//   text that names that receiver (see implicitReceiverText); when no text
//   names it, the call is not reported;
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
//   implicit receiver (`with(s) { uppercase() }`, `s.run { uppercase() }`,
//   an extension body), through an import alias
//   (`import kotlin.text.uppercase as up`) and spelled with backticks
//   (`s.`uppercase`()`). The ASCII-invariant exemption, a user opt-out,
//   still applies to the implicit-receiver calls, so
//   `with(currencyCode) { uppercase() }` is not reported.
internal object UpperLowerInvariantMisuse : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UpperLowerInvariantMisuse"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UpperLowerInvariantMisuse)
    }

    private val kotlinText = FqName("kotlin.text")
    private val kotlinPackage = FqName("kotlin")

    // Stdlib scope functions whose lambda's receiver is the call's explicit
    // receiver (`with` takes it as its first argument instead).
    private val receiverLambdaScopes = setOf("run", "apply")
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
        val receiverText = if (qualified != null) {
            val receiver = significantChildren(qualified, source.treeStructure).firstOrNull() ?: return
            source.treeStructure.toString(receiver).toString()
        } else {
            implicitReceiverText(expression) ?: return
        }
        if (asciiInvariantIdentifiers.any { receiverText.contains(it) }) return
        report(qualified?.let { sourceOf(it, source) } ?: source, message(name))
    }

    // The text that names the implicit receiver of [call], for the
    // ASCII-invariant exemption Go applies to receiver text:
    // - in the lambda of a stdlib scope function, the receiver argument as
    //   written: `currencyCode` for `with(currencyCode) { .. }`,
    //   `currencyCode.run { .. }` and `currencyCode.apply { .. }`;
    // - in an extension function or property, its name, the label of its
    //   receiver (`this@hexUpper` in `fun String.hexUpper()`).
    // Null when nothing names the receiver (another lambda with receiver, a
    // scope function called on an implicit receiver).
    context(context: CheckerContext)
    private fun implicitReceiverText(call: FirFunctionCall): String? {
        var receiver = call.extensionReceiver ?: return null
        while (receiver is FirSmartCastExpression) receiver = receiver.originalExpression
        val thisReceiver = receiver as? FirThisReceiverExpression ?: return null
        val parameter = thisReceiver.calleeReference.boundSymbol as? FirReceiverParameterSymbol ?: return null
        return when (val owner = parameter.containingDeclarationSymbol) {
            is FirAnonymousFunctionSymbol -> scopeFunctionReceiverText(owner)
            is FirNamedFunctionSymbol -> owner.name.asString()
            is FirPropertySymbol -> owner.name.asString()
            else -> null
        }
    }

    // The receiver argument of the stdlib scope function call that takes
    // [lambda], as written.
    context(context: CheckerContext)
    private fun scopeFunctionReceiverText(lambda: FirAnonymousFunctionSymbol): String? {
        val call = context.callsOrAssignments.asReversed().firstNotNullOfOrNull { statement ->
            (statement as? FirFunctionCall)?.takeIf { candidate ->
                candidate.argumentList.arguments.any { it.passesLambda(lambda) }
            }
        } ?: return null
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return null
        if (callableId.packageName != kotlinPackage || callableId.className != null) return null
        val receiver = when (callableId.callableName.asString()) {
            "with" -> call.argumentList.arguments.firstOrNull { !it.passesLambda(lambda) }
            in receiverLambdaScopes -> call.explicitReceiver
            else -> null
        }
        return receiver?.source?.text?.toString()
    }

    private fun FirExpression.passesLambda(lambda: FirAnonymousFunctionSymbol): Boolean {
        val value = if (this is FirWrappedArgumentExpression) expression else this
        return value is FirAnonymousFunctionExpression && value.anonymousFunction.symbol == lambda
    }

    private fun message(name: String) =
        "'$name()' called without explicit Locale. Pass 'Locale.ROOT' for case-insensitive comparison or use a display-locale variant for user-facing text."

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
