package dev.krit.fir.checkers

import dev.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.FqName

internal object InjectDispatcher : FirFunctionCallChecker(MppCheckerKind.Common) {
    private val dispatcherProperties = mapOf(
        FqName("kotlinx.coroutines.Dispatchers.IO") to "IO",
        FqName("kotlinx.coroutines.Dispatchers.Default") to "Default",
        FqName("kotlinx.coroutines.Dispatchers.Unconfined") to "Unconfined",
        FqName("kotlinx.coroutines.Dispatchers.Main") to "Main",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (isIdiomaticDispatcherHost(expression)) return

        for (argument in expression.argumentList.arguments) {
            val dispatcher = hardcodedDispatcherArgument(argument) ?: continue
            if (dispatcher.name == "Main") continue
            reporter.reportOn(dispatcher.source, KritDiagnostics.INJECT_DISPATCHER, dispatcher.name)
        }
    }

    private fun hardcodedDispatcherArgument(argument: FirExpression): DispatcherArgument? {
        val unwrapped = unwrapArgument(argument)
        val access = unwrapped as? FirPropertyAccessExpression ?: return null
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return null
        val fqName = symbol.callableId?.asSingleFqName() ?: return null
        val dispatcherName = dispatcherProperties[fqName] ?: return null
        val source = access.source ?: argument.source ?: return null
        return DispatcherArgument(dispatcherName, source)
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
