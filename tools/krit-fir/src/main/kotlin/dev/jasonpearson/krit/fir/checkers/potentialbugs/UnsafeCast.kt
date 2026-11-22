package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isProvablyImpossibleCast
import dev.jasonpearson.krit.fir.support.isUnsafeCastScaffolding
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall

internal object UnsafeCast : FirTypeOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnsafeCast"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val typeOperatorCallCheckers = setOf(UnsafeCast)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind || isInTestFile()) return
        if (expression.operation !in setOf(FirOperation.AS, FirOperation.SAFE_AS)) return
        if (isUnsafeCastScaffolding(source, config()) || !isProvablyImpossibleCast(expression)) return
        val message = if (expression.operation == FirOperation.SAFE_AS) {
            "Safe cast can never succeed and always returns null."
        } else {
            "Cast can never succeed."
        }
        report(source, message)
    }
}
