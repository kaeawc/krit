package dev.jasonpearson.krit.fir.checkers.i18n

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.toKtLightSourceElement
import org.jetbrains.kotlin.util.getChildren

// Flags `withLocale(Locale.getDefault())` on a machine-readable
// java.time.format.DateTimeFormatter: a formatter derived from one of its
// ISO_* / RFC_* / BASIC_ISO_* constants. Such output is persisted or sent over
// the network, so it should use Locale.ROOT (or Locale.US), not the device
// locale.
//
// Mirrors the Go rule's evidence on top of FIR resolution:
// - the call resolves to DateTimeFormatter.withLocale;
// - its single argument is java.util.Locale.getDefault() with no arguments
//   (the `getDefault(Locale.Category)` overload is not flagged, and neither is
//   a Locale held in a variable, as in Go);
// - the receiver chain starts with one of the formatter constants. Go tests
//   that the receiver's text starts with `DateTimeFormatter.ISO_` (or `RFC_`,
//   `BASIC_ISO_`), so any chain rooted at the constant counts
//   (`DateTimeFormatter.ISO_INSTANT.withZone(utc)`, `?.`, `!!`); FIR walks the
//   explicit receivers down to the chain's leftmost root the same way;
// - the finding is reported on the line where the whole qualified call starts
//   (the receiver's first line), as Go reports on its call_expression.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go matches the spelled text `Locale.getDefault()` and
//   `DateTimeFormatter.ISO_...` once the file mentions
//   java.time.format.DateTimeFormatter. A project `Locale.getDefault()` or a
//   project `DateTimeFormatter` holder is not the device default locale or a
//   java.time ISO constant, so FIR does not report it
//   (LocaleGetDefaultForFormattingLookalike.kt).
// - Recall: FIR also reports the same calls spelled in ways Go's text match
//   misses: import aliases, static imports of the constant or of getDefault,
//   and a parenthesized receiver or argument
//   (LocaleGetDefaultForFormattingRecall.kt).
internal object LocaleGetDefaultForFormatting : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LocaleGetDefaultForFormatting"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LocaleGetDefaultForFormatting)
    }

    private val dateTimeFormatter = ClassId(FqName("java.time.format"), Name.identifier("DateTimeFormatter"))
    private val withLocale = CallableId(dateTimeFormatter, Name.identifier("withLocale"))
    private val localeGetDefault = CallableId(
        ClassId(FqName("java.util"), Name.identifier("Locale")),
        Name.identifier("getDefault"),
    )
    private val formatterConstantPrefixes = listOf("ISO_", "RFC_", "BASIC_ISO_")

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    private const val MESSAGE =
        "'withLocale(Locale.getDefault())' on a persistence/network formatter; pass Locale.ROOT (or Locale.US) so output is locale-independent."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != withLocale) return
        val receiver = expression.explicitReceiver ?: return
        if (!isFormatterConstant(chainRoot(receiver))) return
        val argument = expression.argumentList.arguments.singleOrNull() ?: return
        if (!isLocaleGetDefault(argument)) return

        val source = expression.source ?: return
        val qualified = qualifiedCall(source)
        report(qualified?.let { sourceOf(it, source) } ?: source, MESSAGE)
    }

    // The leftmost expression of a receiver chain: `DateTimeFormatter.ISO_INSTANT`
    // for `DateTimeFormatter.ISO_INSTANT?.withZone(utc)!!`, matching Go's
    // prefix test on the receiver text.
    private fun chainRoot(expression: FirExpression): FirExpression {
        var current = expression
        while (true) {
            current = when (current) {
                is FirSmartCastExpression -> current.originalExpression
                is FirCheckedSafeCallSubject -> current.originalReceiverRef.value
                is FirSafeCallExpression -> current.receiver
                is FirCheckNotNullCall -> current.argumentList.arguments.singleOrNull() ?: return current
                is FirQualifiedAccessExpression -> {
                    val next = current.explicitReceiver
                    if (next == null || next is FirResolvedQualifier) return current
                    next
                }
                else -> return current
            }
        }
    }

    // A DateTimeFormatter ISO_* / RFC_* / BASIC_ISO_* constant.
    private fun isFormatterConstant(expression: FirExpression): Boolean {
        val access = expression as? FirPropertyAccessExpression ?: return false
        val id = access.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        if (id.classId != dateTimeFormatter) return false
        val name = id.callableName.asString()
        return formatterConstantPrefixes.any { name.startsWith(it) }
    }

    // `Locale.getDefault()`, the no-argument overload.
    private fun isLocaleGetDefault(argument: FirExpression): Boolean {
        val value = if (argument is FirWrappedArgumentExpression) argument.expression else argument
        val call = value as? FirFunctionCall ?: return false
        val id = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        return id == localeGetDefault && call.argumentList.arguments.isEmpty()
    }

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call. K2 gives a dot call the whole qualified expression as its source;
    // a safe call keeps the selector call expression, so step up to its parent.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val tree = source.treeStructure
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = tree.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        return parent.takeIf { parent.getChildren(tree).lastOrNull { it.tokenType == KtNodeTypes.CALL_EXPRESSION } == node }
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
}
