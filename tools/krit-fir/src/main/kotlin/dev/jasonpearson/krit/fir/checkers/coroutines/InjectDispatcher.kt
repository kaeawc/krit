package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.name.FqName

internal object InjectDispatcher : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "InjectDispatcher"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(InjectDispatcher)
    }
    private val dispatcherProperties = mapOf(
        FqName("kotlinx.coroutines.Dispatchers.IO") to "IO",
        FqName("kotlinx.coroutines.Dispatchers.Default") to "Default",
        FqName("kotlinx.coroutines.Dispatchers.Unconfined") to "Unconfined",
        FqName("kotlinx.coroutines.Dispatchers.Main") to "Main",
    )

    private val contextPackages = setOf(FqName("kotlin.coroutines"), FqName("kotlinx.coroutines"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        // `job + Dispatchers.IO` is itself a call (CoroutineContext.plus). Its
        // operands are judged at the call the combined context is passed to,
        // so the host exemptions below see the real host and not `plus`.
        if (isContextPlus(expression)) return
        if (isIdiomaticDispatcherHost(expression)) return
        if (!hasDispatchableOwner()) return

        for (argument in expression.argumentList.arguments) {
            for (dispatcher in hardcodedDispatchers(unwrapArgument(argument))) {
                if (dispatcher.name == "Main") continue
                report(dispatcher.source, "Hardcoded Dispatchers.${dispatcher.name}. Inject dispatchers for better testability.")
            }
        }
    }

    // Only flag dispatchers used inside a class/object member, where a dispatcher
    // could realistically be injected via the constructor. Top-level functions and
    // extension functions with no class owner have nothing to inject into, so a
    // hardcoded dispatcher there is not actionable and would be a false positive.
    context(context: CheckerContext)
    private fun hasDispatchableOwner(): Boolean {
        // An enclosing class/object provides a constructor to inject into.
        if (context.containingDeclarations.any { it is FirClassSymbol<*> }) return true

        // Otherwise, the nearest enclosing function decides. A member function has a
        // dispatch receiver; a top-level or extension-without-class function does not.
        val enclosingFunction = context.containingDeclarations
            .filterIsInstance<FirNamedFunctionSymbol>()
            .firstOrNull() ?: return false

        // Extension functions (receiverParameterSymbol != null) and top-level
        // functions (no dispatch receiver, i.e. no owning class) are not injectable.
        if (enclosingFunction.receiverParameterSymbol != null) return false
        return enclosingFunction.dispatchReceiverType != null
    }

    // The hardcoded dispatchers an argument value passes: the value itself,
    // or the operands of a `+` chain combining it with other context elements
    // (`SupervisorJob() + Dispatchers.IO`, `Dispatchers.IO + handler`), in
    // source order.
    private fun hardcodedDispatchers(value: FirExpression): List<DispatcherArgument> {
        if (value is FirFunctionCall && isContextPlus(value)) {
            val receiver = value.explicitReceiver ?: value.dispatchReceiver ?: value.extensionReceiver
            return listOfNotNull(receiver).flatMap { hardcodedDispatchers(it) } +
                value.argumentList.arguments.flatMap { hardcodedDispatchers(unwrapArgument(it)) }
        }
        val access = value as? FirPropertyAccessExpression ?: return emptyList()
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return emptyList()
        val fqName = symbol.callableId?.asSingleFqName() ?: return emptyList()
        val dispatcherName = dispatcherProperties[fqName] ?: return emptyList()
        val source = access.source ?: return emptyList()
        return listOf(DispatcherArgument(dispatcherName, source))
    }

    // `plus` on a coroutine context or scope: CoroutineContext.plus and its
    // overrides (kotlin.coroutines), CoroutineScope.plus (kotlinx.coroutines).
    private fun isContextPlus(expression: FirFunctionCall): Boolean {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return false
        if (callee.name.asString() != "plus") return false
        return callee.callableId?.packageName in contextPackages
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    private fun isIdiomaticDispatcherHost(expression: FirFunctionCall): Boolean {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return false
        val method = callee.name.asString()
        return when (method) {
            "flowOn", "shareIn", "CoroutineScope" -> true
            "async" -> receiverName(expression) == "viewModelScope"
            "launch" -> receiverName(expression) == "viewModelScope" || receiverName(expression) == "lifecycleScope"
            "launchWhenCreated", "launchWhenStarted", "launchWhenResumed" -> receiverName(expression) == "lifecycleScope"
            else -> false
        }
    }

    private fun receiverName(expression: FirFunctionCall): String? {
        val receiver = expression.explicitReceiver as? FirPropertyAccessExpression ?: return null
        return receiver.calleeReference.toResolvedCallableSymbol()?.name?.asString()
    }

    private data class DispatcherArgument(
        val name: String,
        val source: org.jetbrains.kotlin.KtSourceElement,
    )
}
