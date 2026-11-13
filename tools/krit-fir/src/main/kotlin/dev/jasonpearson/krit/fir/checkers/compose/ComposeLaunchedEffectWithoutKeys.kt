package dev.jasonpearson.krit.fir.checkers.compose

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.unwrapArgument
import com.intellij.lang.LighterASTNode
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionTypeConversionExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.isSomeFunctionType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.text

// Flags `LaunchedEffect(Unit) { f(param) }`: an effect keyed only on constants whose body reads a
// parameter of the enclosing function. The effect never restarts when that parameter changes across
// recomposition, so it keeps working with the first value.
//
// This mirrors the Go ComposeLaunchedEffectWithoutKeys rule. Like Go, it stays quiet when the body is
// a one-shot (a top-level `if`, or a call named show*, track, analytics, register, toast, snackbar,
// impression or log), when a comment right above the effect calls it one-shot, when the parameter is
// a callback (function type, `onFoo`, or a name ending in sink, callback, listener, handler, action),
// and when the only reads are method-call receivers (`controller.start()`). Once one parameter is
// reported the message names every captured parameter, receivers included, in source order.
//
// Resolution replaces Go's name matching: the call must resolve to Compose's LaunchedEffect, and a
// parameter counts when a resolved reference points at its symbol, so a shadowing local or lambda
// parameter is not a read.
internal object ComposeLaunchedEffectWithoutKeys : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposeLaunchedEffectWithoutKeys"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ComposeLaunchedEffectWithoutKeys)
    }
    private val launchedEffectFqName = FqName("androidx.compose.runtime.LaunchedEffect")
    private val constantKeys = setOf("Unit", "true", "false", "null")
    private val callbackSuffixes = listOf("sink", "callback", "listener", "handler", "action", "actions")
    private val onceMarkers = listOf("impression", "track", "analytics", "register", "toast", "snackbar", "log")
    private val oneShotPhrases = listOf("one-shot", "one shot", "oneshot", "only once", "runs once", "run once", "once on", "fires once")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId?.asSingleFqName() != launchedEffectFqName) return

        val arguments = expression.argumentList.arguments
        val lambda = trailingLambda(arguments.lastOrNull() ?: return) ?: return
        if (!arguments.dropLast(1).all(::isConstantKey)) return

        val function = context.containingElements.filterIsInstance<FirNamedFunction>().lastOrNull() ?: return
        if (isComposeKeyRuleExempt(function)) return
        // One-shot effects and callback reads are intentional with constant keys.
        if (hasOnceOnlyShape(lambda) || documentedOneShot(source)) return

        val reads = readParameters(lambda, function)
        // A receiver-only parameter alone is not reported, but once one is, the key list names every captured parameter.
        if (reads.none { it.dataRead }) return
        val names = reads.map { it.symbol.name.asString() }
        val list = names.joinToString(", ")
        val noun = if (names.size == 1) "parameter " else "parameters "
        val verb = if (names.size == 1) " changes" else " change"
        report(source, "LaunchedEffect body reads enclosing $noun$list but is keyed only on constants; the effect won't re-run when $list$verb. Pass LaunchedEffect($list) { ... }.")
    }

    private fun trailingLambda(last: FirExpression): FirAnonymousFunctionExpression? {
        val argument = if (last is FirFunctionTypeConversionExpression) last.expression else last
        return (argument as? FirAnonymousFunctionExpression)?.takeIf { it.isTrailingLambda }
    }

    // Go compares the argument text, so only the spellings `Unit`, `true`, `false` and `null` are constants.
    private fun isConstantKey(argument: FirExpression): Boolean {
        if (argument is FirVarargArgumentsExpression) return argument.arguments.all(::isConstantKey)
        return argument.source?.text?.toString()?.trim() in constantKeys
    }

    // The statements of the effect body, looking through the implicit return of the last one.
    private fun statements(lambda: FirAnonymousFunctionExpression): List<FirElement> =
        lambda.anonymousFunction.body?.statements.orEmpty().map { statement ->
            if (statement is FirReturnExpression && statement.source?.kind is KtFakeSourceElementKind) statement.result else statement
        }

    private fun hasOnceOnlyShape(lambda: FirAnonymousFunctionExpression): Boolean {
        val guarded = statements(lambda).any { it is FirWhenExpression && it.source?.elementType == KtNodeTypes.IF }
        if (guarded) return true
        var once = false
        lambda.anonymousFunction.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirFunctionCall && calledNames(element).any(::namesOnceOnlyCall)) once = true
                element.acceptChildren(this)
            }
        })
        return once
    }

    // The written callee name; a call through an `invoke` receiver keeps the receiver's name too.
    private fun calledNames(call: FirFunctionCall): List<String> {
        val name = call.calleeReference.name.asString()
        val receiver = (call.explicitReceiver as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()?.name?.asString()
        return if (name == "invoke" && receiver != null) listOf(name, receiver) else listOf(name)
    }

    // show* calls are host-state notifications such as a snackbar or dialog.
    private fun namesOnceOnlyCall(name: String): Boolean {
        val lower = name.lowercase()
        return lower.startsWith("show") || onceMarkers.any { it in lower }
    }

    // True when comments directly above the effect, or above its first-statement block, call it one-shot.
    private fun documentedOneShot(source: KtSourceElement): Boolean {
        var node = source.lighterASTNode
        var hopped = false
        while (true) {
            val parent = source.treeStructure.getParent(node) ?: return false
            val siblings = lightChildren(source, parent)
            val comments = precedingComments(source, siblings, siblings.indexOf(node)) ?: return false
            if (comments.any { text -> oneShotPhrases.any { it in text.lowercase() } }) return true
            // A comment before the first statement hangs on the lambda, outside its block.
            if (hopped || parent.tokenType != KtNodeTypes.BLOCK) return false
            node = parent
            hopped = true
        }
    }

    // The comment texts directly before [index], nearest first; null when the node is not in the sibling list.
    private fun precedingComments(source: KtSourceElement, siblings: List<LighterASTNode>, index: Int): List<String>? {
        if (index < 0) return null
        val out = ArrayList<String>()
        for (i in index - 1 downTo 0) {
            val sibling = siblings[i]
            when {
                sibling.tokenType == KtTokens.WHITE_SPACE -> continue
                sibling.tokenType in KtTokens.COMMENTS -> out += lightText(source, sibling)
                else -> break
            }
        }
        return out
    }

    private class Read(val symbol: FirValueParameterSymbol, var dataRead: Boolean)

    // The non-callback parameters of [function] the effect reads, in order of first read.
    context(context: CheckerContext)
    private fun readParameters(lambda: FirAnonymousFunctionExpression, function: FirNamedFunction): List<Read> {
        val parameters = function.symbol.valueParameterSymbols.toSet()
        val reads = LinkedHashMap<FirValueParameterSymbol, Read>()
        val receivers = HashSet<FirElement>()
        lambda.anonymousFunction.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirFunctionCall) markReceivers(element, receivers)
                // `x?.f()` keeps x in the safe-call node; the selector call only sees a checked subject.
                if (element is FirSafeCallExpression && element.selector is FirFunctionCall) {
                    receivers += element.receiver.unwrapSmartCast()
                }
                val symbol = (element as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
                if (symbol is FirValueParameterSymbol && symbol in parameters) {
                    val read = reads.getOrPut(symbol) { Read(symbol, false) }
                    if (element !in receivers) read.dataRead = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return reads.values.filterNot { isCallback(it.symbol, context) }
    }

    // `x.f()` reads x only as a receiver; so does `x.callback()`, where x holds the invoked property.
    private fun markReceivers(call: FirFunctionCall, receivers: MutableSet<FirElement>) {
        val receiver = call.explicitReceiver?.unwrapSmartCast() ?: return
        val source = call.source
        if (source != null && qualifiedCall(source) != null) receivers += receiver
        // An implicit invoke keeps the written name, so it matches the name of the property being invoked.
        val invoked = (receiver as? FirQualifiedAccessExpression)?.takeIf { it !is FirFunctionCall } ?: return
        if (invoked.calleeReference.toResolvedCallableSymbol()?.name == call.calleeReference.name) {
            invoked.explicitReceiver?.unwrapSmartCast()?.let(receivers::add)
        }
    }

    private fun FirExpression.unwrapSmartCast(): FirExpression =
        unwrapArgument(if (this is FirSmartCastExpression) originalExpression else this)

    // A callback is read through rememberUpdatedState, so a constant key is fine for it.
    private fun isCallback(parameter: FirValueParameterSymbol, context: CheckerContext): Boolean {
        val name = parameter.name.asString()
        if (name.length > 2 && name.startsWith("on") && name[2].isUpperCase()) return true
        val lower = name.lowercase()
        if (callbackSuffixes.any { lower.endsWith(it) }) return true
        val written = parameter.resolvedReturnTypeRef.source?.text?.toString().orEmpty()
        return "->" in written || parameter.resolvedReturnType.fullyExpandedType(context.session).isSomeFunctionType(context.session)
    }
}
