package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.hasSuspendFunctionType
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.runsInPlace
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.kdoc.lexer.KDocTokens
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go OkHttpCallExecuteSync rule: a synchronous `okhttp3.Call.execute()` in a suspend function with no
 * sign that the work already runs off the coroutine's thread.
 *
 * Go matches the name `execute` and then guesses the receiver is a call: its type from the oracle when there is
 * one, else a receiver written as `...call`, `okhttp...` or declared `Call`. The checker reads the resolved
 * callee, so the call must be a member of `okhttp3.Call` or of a class that implements it, whatever the receiver
 * is named, and an unrelated `execute()` on a variable named `...Call` is not reported.
 *
 * Context is read as in Go, from the nearest enclosing named function: it must be `suspend`. A lambda between
 * the call and that function that the compiler does not run in place (a plain function-typed argument such as
 * `Thread { }` or a stored callback) is not the coroutine's thread, so the call is not reported there; Go ignores
 * lambdas. A suspend lambda or a lambda of an inline function still counts as the function's own body.
 *
 * Go's exemptions are text checks, kept as text checks on the enclosing calls up to that function:
 * a dispatcher or scheduler marker (`Dispatchers.IO`, `Dispatchers.Default`, `Schedulers.io()`, a name containing
 * `iodispatcher`, ...), an executor, a Rx `fromCallable` under `subscribeOn(Schedulers.io())`,
 * a `suspendCancellableCoroutine` bridge that cancels the call, and a KDoc on the function that tells callers to use
 * an IO dispatcher. Go reads the whole text of each enclosing call and lambda; here an enclosing call's text leaves
 * out the receiver or argument that holds the checked call, so a marker in a sibling statement
 * (`coroutineScope { launch(Dispatchers.IO) { }; call.execute() }`) does not exempt it. An injected IO or default
 * dispatcher (`appDispatchers.io`) is read from the resolved member access, not the text, so a comment or string
 * naming one does not exempt the call.
 */
internal object OkHttpCallExecuteSync : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "OkHttpCallExecuteSync"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(OkHttpCallExecuteSync)
    }

    private const val MESSAGE =
        "Synchronous OkHttp Call.execute() in a suspend function has no IO dispatcher evidence; wrap the call in " +
            "withContext(Dispatchers.IO) or use an async OkHttp/Retrofit adapter."

    private val call = classIdOf("okhttp3", "Call")
    private val execute = Name.identifier("execute")
    private val submit = Name.identifier("submit")
    private val cancellableBridge = Name.identifier("suspendCancellableCoroutine")

    private val backgroundMarkers = listOf(
        "Dispatchers.IO",
        "Dispatchers.Default",
        "Schedulers.io()",
        "Schedulers.computation()",
        "subscribeOn(Schedulers.io",
        "subscribeOn(Schedulers.computation",
        "SimpleTask.run",
        "executeOnExecutor(",
    )
    // Lower case.
    private val backgroundNames = listOf("iodispatcher", "databasedispatcher", "backgrounddispatcher")
    private val injectedDispatcherMembers = setOf("io", "default")
    private val rxSources = listOf("fromCallable", "fromAction", "fromRunnable")
    private val rxSchedulers = listOf("subscribeOn(Schedulers.io", "subscribeOn(Schedulers.computation")
    private val kdocMarkers = listOf("dispatchers.io", "io dispatcher", "background thread", "iodispatcher")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (!isCallExecute(expression)) return
        val elements = context.containingElements
        val functionIndex = elements.indexOfLast { it is FirNamedFunction }
        val function = elements.getOrNull(functionIndex) as? FirNamedFunction ?: return
        if (!function.symbol.isSuspend) return
        if (!runsOnCoroutineThread(elements, functionIndex)) return
        if (insideBackgroundBoundary(expression, elements, functionIndex, function)) return
        if (isCancellableBridge(elements)) return
        if (function.source?.docMentionsIoDispatcher() == true) return
        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isCallExecute(expression: FirFunctionCall): Boolean {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name != execute || callee.valueParameterSymbols.isNotEmpty()) return false
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        return isOrInheritsFrom(owner, call, context.session, lookupInterfaces = true)
    }

    // A lambda the compiler runs elsewhere (not suspend, not inline) hides the call from the coroutine's thread.
    private fun runsOnCoroutineThread(elements: List<FirElement>, functionIndex: Int): Boolean =
        (functionIndex + 1 until elements.size).all { index ->
            val lambda = elements[index] as? FirAnonymousFunction
            lambda == null || hasSuspendFunctionType(lambda) || runsInPlace(elements, index, lambda)
        }

    private fun insideBackgroundBoundary(
        expression: FirFunctionCall,
        elements: List<FirElement>,
        functionIndex: Int,
        function: FirNamedFunction,
    ): Boolean {
        val functionText by lazy { function.source?.let { lightText(it, it.lighterASTNode) }.orEmpty() }
        val at = expression.source?.startOffset ?: return false
        for (index in elements.indices.reversed()) {
            if (index <= functionIndex) break
            val element = elements[index]
            // A lambda's own text is its body, which holds the call and its siblings, not the context it runs in.
            if (element === expression || element !is FirFunctionCall) continue
            val source = element.source ?: continue
            if (marksBackground(textAround(element, source, at), functionText)) return true
            if (passesInjectedDispatcher(element, at)) return true
            if (isExecutorSubmit(element)) return true
        }
        return false
    }

    // The text Go sees for a call: a dot call includes its receiver chain, which K2 keeps outside a safe call.
    private fun enclosingText(source: KtSourceElement): String =
        lightText(source, qualifiedCall(source) ?: source.lighterASTNode)

    // [enclosingText] without the receiver or argument that holds offset [at]: a marker beside the checked call
    // (`coroutineScope { launch(Dispatchers.IO) { }; call.execute() }`) does not move it off the coroutine's thread.
    private fun textAround(call: FirFunctionCall, source: KtSourceElement, at: Int): String {
        val node = qualifiedCall(source) ?: source.lighterASTNode
        val text = lightText(source, node)
        val holder = (listOfNotNull(call.explicitReceiver) + call.argumentList.arguments)
            .mapNotNull { it.source }
            .firstOrNull { at >= it.startOffset && at < it.endOffset } ?: return text
        val from = holder.startOffset - node.startOffset
        val to = holder.endOffset - node.startOffset
        return if (from in 0..<to && to <= text.length) text.removeRange(from, to) else text
    }

    // A receiver or argument of [call], other than the one holding offset [at], that reads an injected IO or
    // default dispatcher: a member `io` or `default` (any case) of a value whose name ends in `dispatchers`.
    private fun passesInjectedDispatcher(call: FirFunctionCall, at: Int): Boolean =
        (listOfNotNull(call.explicitReceiver) + call.argumentList.arguments).any { part ->
            val source = part.source
            (source == null || at < source.startOffset || at >= source.endOffset) && readsInjectedDispatcher(part)
        }

    private fun readsInjectedDispatcher(root: FirElement): Boolean {
        var found = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirPropertyAccessExpression && isInjectedDispatcher(element)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun isInjectedDispatcher(access: FirPropertyAccessExpression): Boolean {
        if (access.calleeReference.name.asString().lowercase() !in injectedDispatcherMembers) return false
        val owner = when (val receiver = access.explicitReceiver) {
            is FirQualifiedAccessExpression -> receiver.calleeReference.toResolvedCallableSymbol()?.name?.asString()
            is FirResolvedQualifier -> receiver.relativeClassFqName?.shortName()?.asString()
            else -> null
        } ?: return false
        return owner.lowercase().endsWith("dispatchers")
    }

    private fun marksBackground(text: String, functionText: String): Boolean {
        if (backgroundMarkers.any { it in text }) return true
        if (".execute" in text && "Executor" in text) return true
        val lower = text.lowercase()
        if (backgroundNames.any { it in lower }) return true
        return rxSources.any { it in text } && rxSchedulers.any { it in functionText }
    }

    // `executor.execute { }` and `backgroundScope.submit { }`: Go reads the receiver name.
    private fun isExecutorSubmit(call: FirFunctionCall): Boolean {
        val name = call.calleeReference.name
        if (name != execute && name != submit) return false
        val receiver = call.explicitReceiver?.source ?: return false
        val lower = lightText(receiver, receiver.lighterASTNode).lowercase()
        return "executor" in lower || "background" in lower
    }

    // The nearest suspendCancellableCoroutine above the call, anywhere up the tree, must cancel the call.
    private fun isCancellableBridge(elements: List<FirElement>): Boolean {
        val bridge = elements.lastOrNull { it is FirFunctionCall && it.calleeReference.name == cancellableBridge }
        val source = bridge?.source ?: return false
        val text = enclosingText(source)
        return "invokeOnCancellation" in text && ".cancel()" in text
    }

    private fun KtSourceElement.docMentionsIoDispatcher(): Boolean =
        lightChildren(this, lighterASTNode).any { child ->
            child.tokenType == KDocTokens.KDOC &&
                lightText(this, child).lowercase().let { text -> kdocMarkers.any { it in text } }
        }
}
