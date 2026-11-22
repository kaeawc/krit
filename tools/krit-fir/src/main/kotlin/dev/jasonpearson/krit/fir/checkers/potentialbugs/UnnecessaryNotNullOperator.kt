package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.oracle.ErrorInvolvement
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirCheckNotNullCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.references.toResolvedPropertySymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens

internal object UnnecessaryNotNullOperator : FirCheckNotNullCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessaryNotNullOperator"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val checkNotNullCallCheckers = setOf(UnnecessaryNotNullOperator)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirCheckNotNullCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val operand = unwrapSmartCasts(expression, throughNotNull = true, throughSafeCallSubject = false)
        if (!definitelyNonNull(operand)) return
        val children = significantChildren(source, source.lighterASTNode)
        val operation = children.firstOrNull { it.tokenType == KtNodeTypes.OPERATION_REFERENCE } ?: return
        val operator = significantChildren(source, operation).firstOrNull { it.tokenType == KtTokens.EXCLEXCL }
            ?: return
        val receiver = children.firstOrNull() ?: return
        val text = lightText(source, receiver).trim()
        report(lightSourceOf(operator, source), message(text))
    }

    context(context: CheckerContext)
    private fun definitelyNonNull(operand: FirExpression): Boolean {
        if (isExemptOperand(operand) || ErrorInvolvement.involvesError(operand)) return false
        val type = operand.resolvedType.fullyExpandedType()
        return type !is ConeFlexibleType && type !is ConeErrorType && !type.canBeNull(context.session)
    }

    context(context: CheckerContext)
    private fun isExemptOperand(operand: FirExpression): Boolean {
        val symbol = (operand as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedPropertySymbol()
        return symbol?.isVar == true || operand is FirAnonymousFunctionExpression ||
            operand is FirThisReceiverExpression && context.containingElements.any { it is FirAnonymousFunction }
    }

    private fun message(text: String): String = if (isSimpleName(text)) {
        "Unnecessary not-null assertion (!!) on non-nullable '$text'."
    } else {
        "Unnecessary not-null assertion (!!): '$text' is already non-null here (non-null type or smart cast)."
    }

    private fun isSimpleName(text: String): Boolean =
        text.firstOrNull()?.let { it == '_' || it in 'a'..'z' || it in 'A'..'Z' } == true &&
            text.all { it == '_' || it in 'a'..'z' || it in 'A'..'Z' || it in '0'..'9' }
}
