package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

internal object CollectInOnCreateWithoutLifecycle : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CollectInOnCreateWithoutLifecycle"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(CollectInOnCreateWithoutLifecycle)
    }
    private val collectFqNames = setOf(
        FqName("kotlinx.coroutines.flow.collect"),
        FqName("kotlinx.coroutines.flow.Flow.collect"),
    )

    // Lifecycle callbacks whose bodies run before the view is STOPPED; a Flow
    // collection started here keeps the upstream active past the lifecycle
    // unless it is restarted with repeatOnLifecycle. Matches the Go
    // CollectInOnCreateWithoutLifecycle rule's callback set.
    private val lifecycleCallbacks = setOf("onCreate", "onStart", "onViewCreated")

    // The calls a collect may sit in and still run for the whole lifecycle:
    // coroutine builders and scope functions of kotlin and kotlinx.coroutines,
    // and the androidx.lifecycle builders that only SUSPEND the block below a
    // state (launchWhenStarted, whenStarted) while keeping the upstream flow
    // subscribed. Any other enclosing call, repeatOnLifecycle or a project
    // wrapper around it, may stop the collection, so the collect is not
    // reported. Go matches the same names.
    private val lifecycleBlindWrappers = setOf(
        "launch", "async", "withContext", "coroutineScope", "supervisorScope", "withTimeout",
        "withTimeoutOrNull", "runBlocking", "let", "run", "apply", "also", "with", "takeIf",
        "takeUnless", "use", "runCatching", "repeat", "forEach", "synchronized",
    )
    private val pausingLifecycleWrappers = setOf(
        "launchWhenCreated", "launchWhenStarted", "launchWhenResumed", "whenCreated", "whenStarted", "whenResumed",
    )
    private val lifecyclePackage = FqName("androidx.lifecycle")

    // flowWithLifecycle stops collecting its upstream below the given state,
    // as repeatOnLifecycle does, so a collect downstream of it is safe.
    private val flowWithLifecycle = CallableId(lifecyclePackage, Name.identifier("flowWithLifecycle"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return

        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (!isFlowCollect(expression, callee)) return
        if (collectsLifecycleAwareFlow(expression)) return

        // The nearest enclosing named function must be a lifecycle callback,
        // and every call between it and the collect must be one that does not
        // follow the lifecycle. Coroutine builders contribute anonymous
        // functions, so the nearest named function is the lifecycle method
        // that lexically contains the collect.
        val enclosingFunction = enclosingFunctionThroughBlindWrappers(expression) ?: return
        if (enclosingFunction.name.asString() !in lifecycleCallbacks) return

        // Go reports on the callee name, so a chain split across lines keeps the `.collect` line.
        report(expression.calleeReference.source ?: source, "Flow.collect inside onCreate/onStart/onViewCreated should be wrapped in repeatOnLifecycle to stop collecting when the lifecycle is stopped.")
    }

    // The nearest named function around [expression], or null when a call the
    // collect is passed into (not one it is the receiver of) may follow the
    // lifecycle.
    context(context: CheckerContext)
    private fun enclosingFunctionThroughBlindWrappers(expression: FirFunctionCall): FirNamedFunction? {
        val path = context.containingElements
        var child: FirElement = expression
        for (i in path.indices.reversed()) {
            val element = path[i]
            if (element === expression) continue
            when (element) {
                is FirNamedFunction -> return element
                is FirFunctionCall -> if (!isReceiverOf(element, child) && !isLifecycleBlind(element)) return null
                else -> {}
            }
            child = element
        }
        return null
    }

    private fun isReceiverOf(call: FirFunctionCall, child: FirElement): Boolean =
        child === call.explicitReceiver || child === call.dispatchReceiver || child === call.extensionReceiver

    private fun isLifecycleBlind(call: FirFunctionCall): Boolean {
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        val name = callableId.callableName.asString()
        val packageName = callableId.packageName.asString()
        if (name in pausingLifecycleWrappers) return callableId.packageName == lifecyclePackage
        if (name !in lifecycleBlindWrappers) return false
        return packageName == "kotlin" || packageName.startsWith("kotlin.") ||
            packageName == "kotlinx.coroutines" || packageName.startsWith("kotlinx.coroutines.")
    }

    private fun collectsLifecycleAwareFlow(expression: FirFunctionCall): Boolean {
        var receiver: FirExpression? = expression.explicitReceiver
        while (receiver is FirFunctionCall) {
            if (receiver.calleeReference.toResolvedCallableSymbol()?.callableId == flowWithLifecycle) return true
            receiver = receiver.explicitReceiver
        }
        return false
    }

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
