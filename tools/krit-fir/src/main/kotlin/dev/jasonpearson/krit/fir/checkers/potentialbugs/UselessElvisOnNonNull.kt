package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isDefinitelyNonNullOperand
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirElvisExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression

internal object UselessElvisOnNonNull : FirElvisExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UselessElvisOnNonNull"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val elvisExpressionCheckers = setOf(UselessElvisOnNonNull)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirElvisExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind || !expression.lhs.isDefinitelyNonNullOperand()) return
        val left = significantChildren(source, source.lighterASTNode).firstOrNull() ?: return
        val leftText = lightText(source, left).trim()
        report(source, "Useless elvis (?:) on non-nullable '$leftText'. The fallback is dead code.")
    }
}
