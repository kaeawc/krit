package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.trailingLambdaOf
import dev.jasonpearson.krit.fir.support.walkWithParents
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol

internal object ClickableTouchListener : FirFunctionCallChecker(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (!ClickableTouchSymbols.setter(callee)) return
        val lambda = expression.argumentList.arguments.lastOrNull()?.let(::trailingLambdaOf)
        if (lambda != null) {
            if (!ClickableTouchBodies.callsClick(lambda)) {
                ClickableViewAccessibility.report(
                    lambda.source,
                    "`onTouch` lambda should call `View#performClick` when a click is detected",
                )
            }
        } else {
            checkObjects(expression)
        }
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun checkObjects(expression: FirFunctionCall) {
        walkWithParents(expression.argumentList) { element, _ ->
            if (element is FirNamedFunction && ClickableTouchSymbols.touchOverride(element.symbol)) {
                reportTouch(element)
            }
        }
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun reportTouch(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind || ClickableTouchBodies.callsClick(declaration)) return
        ClickableViewAccessibility.report(
            identifierAnchor(source) ?: source,
            "`onTouch` should call `View#performClick` when a click is detected",
        )
    }
}
