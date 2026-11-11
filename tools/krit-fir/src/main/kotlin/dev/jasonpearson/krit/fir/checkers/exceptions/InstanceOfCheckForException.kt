package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol

/**
 * Flags `is`, `!is` and unsafe `as` applied to the parameter of an enclosing `catch`: a further
 * `catch` clause does the same without the check. `as?` and `when (e) { is X -> }` are not flagged.
 *
 * Go matches the checked name against the caught variable's spelling inside the catch body. Here the
 * operand must resolve to the catch parameter itself, so a lambda parameter, local or property that
 * reuses the name is not flagged. Go also misses a parenthesised operand, which FIR reports, since
 * the checked value is still the caught exception.
 */
internal object InstanceOfCheckForException : FirTypeOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "InstanceOfCheckForException"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val typeOperatorCallCheckers = setOf(InstanceOfCheckForException)
    }

    private const val MESSAGE =
        "Instead of catching for a general exception type and checking for a specific exception type, use multiple catch blocks."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        when (expression.operation) {
            FirOperation.IS, FirOperation.NOT_IS, FirOperation.AS -> Unit
            else -> return
        }
        val operand = expression.argumentList.arguments.singleOrNull() ?: return
        if (!isCaughtParameter(operand)) return
        report(expression.source ?: return, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isCaughtParameter(operand: FirExpression): Boolean {
        var value = operand
        while (value is FirSmartCastExpression) value = value.originalExpression
        val access = value as? FirPropertyAccessExpression ?: return false
        if (access.explicitReceiver != null) return false
        val variable = access.calleeReference.toResolvedVariableSymbol() ?: return false
        return context.containingElements.any { it is FirCatch && it.parameter.symbol == variable }
    }
}
