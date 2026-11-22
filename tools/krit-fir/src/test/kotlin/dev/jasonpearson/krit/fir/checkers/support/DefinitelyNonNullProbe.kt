package dev.jasonpearson.krit.fir.checkers.support

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isDefinitelyNonNullOperand
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirElvisExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression

object DefinitelyNonNullProbe : FirElvisExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "DefinitelyNonNullProbe"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val elvisExpressionCheckers = setOf(DefinitelyNonNullProbe)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirElvisExpression) {
        report(expression.source, expression.lhs.isDefinitelyNonNullOperand().toString())
    }
}
