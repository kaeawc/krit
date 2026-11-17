package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSubtypeOf

/**
 * Flags `is`, `!is` and unsafe `as` applied to the parameter of an enclosing `catch`: a further
 * `catch` clause does the same without the check. `as?` and `when (e) { is X -> }` are not flagged.
 *
 * Go matches the checked name against the caught variable's spelling inside the catch body. Here the
 * operand must resolve to the catch parameter itself, so a lambda parameter, local or property that
 * reuses the name is not flagged. Go also misses a parenthesised operand, which FIR reports, since
 * the checked value is still the caught exception.
 *
 * As in detekt, the checked type must be a subtype of the caught type: only then can a catch clause
 * replace the check. Go, without types, also reports interface checks and upcasts. When either type
 * is unresolved the check is reported, as Go does.
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
        val catch = caughtBy(operand) ?: return
        if (!narrowsCaughtType(expression, catch)) return
        report(expression.source ?: return, MESSAGE)
    }

    // The enclosing catch whose parameter [operand] reads.
    context(context: CheckerContext)
    private fun caughtBy(operand: FirExpression): FirCatch? {
        val value = unwrapSmartCasts(operand, throughSafeCallSubject = false)
        val access = value as? FirPropertyAccessExpression ?: return null
        if (access.explicitReceiver != null) return null
        val variable = access.calleeReference.toResolvedVariableSymbol() ?: return null
        return context.containingElements.lastOrNull { it is FirCatch && it.parameter.symbol == variable } as? FirCatch
    }

    context(context: CheckerContext)
    private fun narrowsCaughtType(expression: FirTypeOperatorCall, catch: FirCatch): Boolean {
        val checked = expression.conversionTypeRef.coneType.fullyExpandedType()
        val caught = catch.parameter.returnTypeRef.coneType.fullyExpandedType()
        if (checked is ConeErrorType || caught is ConeErrorType) return true
        return checked.isSubtypeOf(caught, context.session)
    }
}
