package dev.krit.fir.checkers

import dev.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.name.FqName

internal object FlowCollectInOnCreate : FirFunctionCallChecker(MppCheckerKind.Common) {
    private val collectFqName = FqName("kotlinx.coroutines.flow.collect")
    private val safeContainers = setOf("repeatOnLifecycle", "launchWhenStarted", "launchWhenResumed")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return

        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId?.asSingleFqName() != collectFqName) return

        val containingFunctions = context.containingDeclarations.filterIsInstance<FirNamedFunctionSymbol>()

        // Safe if wrapped in a lifecycle-aware scope
        if (containingFunctions.any { it.name.asString() in safeContainers }) return

        // Flag if the outermost function in the call stack is onCreate
        val outerFunction = containingFunctions.lastOrNull() ?: return
        if (outerFunction.name.asString() != "onCreate") return

        reporter.reportOn(source, KritDiagnostics.FLOW_COLLECT_IN_ON_CREATE)
    }
}
