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
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.utils.isFinal
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedBaseSymbol
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.toKtLightSourceElement
import org.jetbrains.kotlin.util.getChildren

// Flags `withLocale(Locale.getDefault())` on a machine-readable
// DateTimeFormatter (java.time.format, or the ThreeTenBP backport
// org.threeten.bp.format): a formatter derived from one of its ISO_* / RFC_* /
// BASIC_ISO_* constants. Such output is persisted or sent over the network, so
// it should use Locale.ROOT (or Locale.US), not the device locale.
//
// Mirrors the Go rule's evidence on top of FIR resolution:
// - the call resolves to DateTimeFormatter.withLocale;
// - its single argument is java.util.Locale.getDefault() with no arguments
//   (the `getDefault(Locale.Category)` overload is not flagged, and neither is
//   a Locale held in a variable, as in Go);
// - the receiver is a formatter derived from one of the constants. Go tests
//   that the receiver's text starts with `DateTimeFormatter.ISO_` (or `RFC_`,
//   `BASIC_ISO_`); FIR traces the receiver back to where its value comes from
//   (see [origin]);
// - the finding is reported on the line where the whole qualified call starts
//   (the receiver's first line), as Go reports on its call_expression.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go matches the spelled texts `Locale.getDefault()` and
//   `DateTimeFormatter.ISO_...` once the file mentions
//   java.time.format.DateTimeFormatter. A project `Locale.getDefault()` is not
//   the device default locale, a project `DateTimeFormatter` holder whose value
//   is not the ISO constant is not a machine-readable formatter, and a scope
//   function or extension that returns an unrelated formatter
//   (`.let { DateTimeFormatter.ofPattern(p) }`) does not pass the ISO formatter
//   on, so FIR does not report them (LocaleGetDefaultForFormattingLookalike.kt,
//   LocaleGetDefaultForFormattingReceiverChain.kt).
// - Recall: FIR also reports the same calls spelled in ways Go's text match
//   misses: import and type aliases, static imports of the constant or of
//   getDefault, parentheses, comments inside the receiver or the argument, a
//   `val` holding the constant, and a ThreeTenBP formatter in a file that does
//   not mention java.time.format.DateTimeFormatter
//   (LocaleGetDefaultForFormattingRecall.kt, LocaleGetDefaultForFormattingTest).
internal object LocaleGetDefaultForFormatting : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LocaleGetDefaultForFormatting"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LocaleGetDefaultForFormatting)
    }

    private val formatterClasses = setOf(
        ClassId(FqName("java.time.format"), Name.identifier("DateTimeFormatter")),
        ClassId(FqName("org.threeten.bp.format"), Name.identifier("DateTimeFormatter")),
    )
    private val withLocale = Name.identifier("withLocale")
    private val localizedBy = Name.identifier("localizedBy")
    private val localeGetDefault = CallableId(
        ClassId(FqName("java.util"), Name.identifier("Locale")),
        Name.identifier("getDefault"),
    )
    private val formatterConstantPrefixes = listOf("ISO_", "RFC_", "BASIC_ISO_")

    // Stdlib scope functions whose result is their receiver.
    private val receiverScopeFunctions = listOf("also", "apply", "takeIf", "takeUnless")
        .mapTo(HashSet()) { CallableId(FqName("kotlin"), Name.identifier(it)) }

    // Stdlib scope functions whose result is their lambda's result.
    private val lambdaScopeFunctions = listOf("let", "run")
        .mapTo(HashSet()) { CallableId(FqName("kotlin"), Name.identifier(it)) }

    // Bounds the receiver trace: chain steps plus initializers and bodies
    // followed, so a cycle of forwarding properties terminates.
    private const val TRACE_BUDGET = 64

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    private const val MESSAGE =
        "'withLocale(Locale.getDefault())' on a persistence/network formatter; pass Locale.ROOT (or Locale.US) so output is locale-independent."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val calleeId = callee.callableId ?: return
        if (calleeId.classId !in formatterClasses || calleeId.callableName != withLocale) return
        val receiver = expression.explicitReceiver ?: return
        if (origin(receiver, emptySet(), TRACE_BUDGET) != Origin.CONSTANT) return
        val argument = expression.argumentList.arguments.singleOrNull() ?: return
        if (!isLocaleGetDefault(argument)) return

        val source = expression.source ?: return
        val qualified = qualifiedCall(source)
        report(qualified?.let { sourceOf(it, source) } ?: source, MESSAGE)
    }

    // Where a formatter value comes from.
    private enum class Origin {
        // An ISO_* / RFC_* / BASIC_ISO_* constant, or a formatter derived from one.
        CONSTANT,

        // The receiver of the lambda or extension body being traced (one of
        // the `holes`): the value is the one the scope function or extension
        // was called on.
        HOLE,

        // Anything else: another formatter, or a value FIR cannot trace.
        OTHER,
    }

    // Traces [expression] back to where its formatter comes from. Go accepts
    // any receiver whose text starts with the constant, so FIR walks the same
    // chains, but only through calls whose result comes from their receiver:
    // - `?.`, `!!`, and smart casts;
    // - DateTimeFormatter's own with*/localizedBy copies;
    // - also/apply/takeIf/takeUnless, which return their receiver;
    // - let/run, and source extension functions and extension properties with
    //   a single-expression body, through their result: `.let { it }` and
    //   `.run { withZone(utc) }` pass the receiver on, while
    //   `.let { DateTimeFormatter.ofPattern(p) }` returns another formatter;
    // - other calls on a receiver that FIR cannot see into (library
    //   extensions, block bodies), as Go does.
    // A final `val` without a custom getter or delegate is traced through its
    // initializer (a forwarding holder counts, a holder of an ofPattern
    // formatter does not), and a final custom getter through its
    // single-expression body. An open property or extension may be overridden
    // with another formatter, so it is not traced.
    private fun origin(expression: FirExpression, holes: Set<FirBasedSymbol<*>>, budget: Int): Origin {
        if (budget <= 0) return Origin.OTHER
        val next = budget - 1
        return when (expression) {
            is FirSmartCastExpression -> origin(expression.originalExpression, holes, next)
            is FirCheckedSafeCallSubject -> origin(expression.originalReceiverRef.value, holes, next)
            is FirSafeCallExpression ->
                (expression.selector as? FirExpression)?.let { origin(it, holes, next) } ?: Origin.OTHER
            is FirCheckNotNullCall ->
                expression.argumentList.arguments.singleOrNull()?.let { origin(it, holes, next) } ?: Origin.OTHER
            is FirWrappedArgumentExpression -> origin(expression.expression, holes, next)
            is FirThisReceiverExpression -> {
                val bound: FirBasedSymbol<*>? = expression.calleeReference.boundSymbol
                if (bound != null && bound in holes) Origin.HOLE else Origin.OTHER
            }
            is FirFunctionCall -> callOrigin(expression, holes, next)
            is FirPropertyAccessExpression -> propertyOrigin(expression, holes, next)
            else -> Origin.OTHER
        }
    }

    private fun callOrigin(call: FirFunctionCall, holes: Set<FirBasedSymbol<*>>, budget: Int): Origin {
        val symbol = call.calleeReference.toResolvedCallableSymbol() ?: return Origin.OTHER
        val id = symbol.callableId
        val receiver = receiverOf(call) ?: return Origin.OTHER
        if (id != null && id.classId in formatterClasses) {
            val name = id.callableName
            val copies = name.asString().startsWith("with") || name == localizedBy
            return if (copies) origin(receiver, holes, budget) else Origin.OTHER
        }
        if (id != null && id in receiverScopeFunctions) return origin(receiver, holes, budget)
        if (id != null && id in lambdaScopeFunctions) {
            val lambda = (call.argumentList.arguments.singleOrNull() as? FirAnonymousFunctionExpression)
                ?.anonymousFunction
            val result = lambda?.let { lambdaResult(it) } ?: return origin(receiver, holes, budget)
            val lambdaHoles = setOfNotNull(
                lambda.symbol,
                lambda.receiverParameter?.symbol,
                lambda.valueParameters.firstOrNull()?.symbol,
            )
            return through(origin(result, lambdaHoles, budget), receiver, holes, budget)
        }
        val function = symbol as? FirNamedFunctionSymbol
        val extensionReceiver = function?.receiverParameterSymbol
        if (extensionReceiver != null && function.isFinal) {
            val result = bodyResult(function)
            if (result != null) {
                val bodyHoles = setOf(function, extensionReceiver)
                return through(origin(result, bodyHoles, budget), receiver, holes, budget)
            }
        }
        return origin(receiver, holes, budget)
    }

    private fun propertyOrigin(
        access: FirPropertyAccessExpression,
        holes: Set<FirBasedSymbol<*>>,
        budget: Int,
    ): Origin {
        if (isFormatterConstant(access)) return Origin.CONSTANT
        val symbol = access.calleeReference.toResolvedBaseSymbol() ?: return Origin.OTHER
        if (symbol in holes) return Origin.HOLE
        val explicit = access.explicitReceiver
        val receiver = if (explicit is FirResolvedQualifier) null else explicit
        if (symbol is FirPropertySymbol && symbol.isFinal) {
            val getter = symbol.getterSymbol
            if (getter == null || getter.isDefault) {
                if (symbol.isVal && !symbol.hasDelegate) {
                    symbol.resolvedInitializer?.let { return origin(it, emptySet(), budget) }
                }
            } else {
                val result = bodyResult(getter)
                if (result != null) {
                    val getterHoles = setOfNotNull(getter, symbol, symbol.receiverParameterSymbol)
                    val traced = origin(result, getterHoles, budget)
                    if (traced != Origin.HOLE) return traced
                    val valueReceiver = receiver ?: access.extensionReceiver ?: access.dispatchReceiver
                    return valueReceiver?.let { origin(it, holes, budget) } ?: Origin.OTHER
                }
            }
        }
        return receiver?.let { origin(it, holes, budget) } ?: Origin.OTHER
    }

    // A scope function or extension whose body traced to [result]: its own
    // receiver ([Origin.HOLE]) continues the trace at the call's [receiver].
    private fun through(result: Origin, receiver: FirExpression, holes: Set<FirBasedSymbol<*>>, budget: Int): Origin =
        if (result == Origin.HOLE) origin(receiver, holes, budget) else result

    // The value a call is made on: its explicit receiver, or the implicit
    // `this` it binds to. A class qualifier (a static call such as
    // `DateTimeFormatter.ofPattern(p)`) is not a value.
    private fun receiverOf(call: FirFunctionCall): FirExpression? {
        val explicit = call.explicitReceiver
        if (explicit is FirResolvedQualifier) return null
        return explicit ?: call.extensionReceiver ?: call.dispatchReceiver
    }

    // The result expression of a lambda: its last statement.
    private fun lambdaResult(lambda: FirAnonymousFunction): FirExpression? =
        unwrapReturn(lambda.body?.statements?.lastOrNull())

    // The result of a source function or getter whose body is a single
    // expression or return; null for a library function or a block body.
    @OptIn(SymbolInternals::class)
    private fun bodyResult(function: FirFunctionSymbol<*>): FirExpression? =
        unwrapReturn(function.fir.body?.statements?.singleOrNull())

    private fun unwrapReturn(statement: FirStatement?): FirExpression? =
        if (statement is FirReturnExpression) statement.result else statement as? FirExpression

    // A DateTimeFormatter ISO_* / RFC_* / BASIC_ISO_* constant.
    private fun isFormatterConstant(expression: FirExpression): Boolean {
        val access = expression as? FirPropertyAccessExpression ?: return false
        val id = access.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        if (id.classId !in formatterClasses) return false
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
