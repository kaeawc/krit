package dev.jasonpearson.krit.fir.checkers

import dev.jasonpearson.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.FqName

internal object FlowCollectInOnCreate : FirFunctionCallChecker(MppCheckerKind.Common) {
    private val collectFqNames = setOf(
        FqName("kotlinx.coroutines.flow.collect"),
        FqName("kotlinx.coroutines.flow.Flow.collect"),
    )

    // Lifecycle callbacks whose bodies run before the view is STOPPED; a Flow
    // collection started here keeps the upstream active past the lifecycle
    // unless it is restarted with repeatOnLifecycle. Matches the Go
    // CollectInOnCreateWithoutLifecycle rule's callback set.
    private val lifecycleCallbacks = setOf("onCreate", "onStart", "onViewCreated")

    // Only repeatOnLifecycle actually cancels and restarts the collection with
    // the lifecycle. launchWhenStarted/launchWhenResumed merely SUSPEND the
    // collector while keeping the upstream flow subscribed, so they do not fix
    // the leak and are not treated as safe (matching the Go rule).
    private const val SAFE_WRAPPER = "repeatOnLifecycle"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return

        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (!isFlowCollect(expression, callee)) return

        // Safe only if an enclosing call is repeatOnLifecycle.
        if (context.callsOrAssignments.filterIsInstance<FirFunctionCall>().any { callName(it) == SAFE_WRAPPER }) return

        // The nearest enclosing named function must be a lifecycle callback.
        // Coroutine builders (launch, repeatOnLifecycle) contribute anonymous
        // functions, so the nearest named function is the lifecycle method
        // that lexically contains the collect.
        val enclosingFunction = context.containingDeclarations
            .filterIsInstance<FirNamedFunctionSymbol>()
            .lastOrNull() ?: return
        if (enclosingFunction.name.asString() !in lifecycleCallbacks) return

        reporter.reportOn(source, KritDiagnostics.FLOW_COLLECT_IN_ON_CREATE)
    }

    private fun callName(call: FirFunctionCall): String? =
        call.calleeReference.toResolvedCallableSymbol()?.name?.asString()

    private fun isFlowCollect(expression: FirFunctionCall, callee: FirCallableSymbol<*>): Boolean {
        if (callee.callableId?.asSingleFqName() in collectFqNames) return true
        if (callee.name.asString() != "collect") return false
        val receivers = listOfNotNull(
            expression.dispatchReceiver,
            expression.extensionReceiver,
            expression.explicitReceiver,
        )
        return receivers.any { receiver ->
            val fqName = receiver.resolvedType.classId?.asSingleFqName()?.asString()
            fqName == "kotlinx.coroutines.flow.Flow" || fqName?.startsWith("kotlinx.coroutines.flow.") == true
        }
    }
}
