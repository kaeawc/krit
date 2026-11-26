package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.oracle.ErrorInvolvement
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isStableInCurrentModule
import dev.jasonpearson.krit.fir.support.reliesOnCrossModuleSmartCast
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirSafeCallExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens

internal object UnnecessarySafeCall : FirSafeCallExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessarySafeCall"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val safeCallExpressionCheckers = setOf(UnnecessarySafeCall)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirSafeCallExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind || !definitelyNonNull(expression.receiver)) return
        val children = significantChildren(source, source.lighterASTNode)
        val operator = children.firstOrNull { it.tokenType == KtTokens.SAFE_ACCESS } ?: return
        val receiver = children.firstOrNull() ?: return
        val text = lightText(source, receiver).trim()
        val simple = text == "this" || receiver.tokenType == KtNodeTypes.REFERENCE_EXPRESSION && isSimpleName(text)
        report(lightSourceOf(operator, source), message(expression.receiver, text, simple))
    }

    context(context: CheckerContext)
    private fun definitelyNonNull(receiver: FirExpression): Boolean {
        if ((receiver as? FirSmartCastExpression)?.isStableInCurrentModule() == false ||
            receiver.reliesOnCrossModuleSmartCast() || ErrorInvolvement.involvesError(receiver)
        ) return false
        val type = receiver.resolvedType.fullyExpandedType()
        return type !is ConeFlexibleType && type !is ConeErrorType && !type.canBeNull(context.session)
    }

    private fun message(receiver: FirExpression, text: String, simple: Boolean): String {
        val symbol = (receiver as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
        return when {
            !simple -> "Unnecessary safe call (?.): '$text' is already non-null here (non-null type or smart cast)."
            symbol is FirValueParameterSymbol -> "Unnecessary safe call (?.) on non-nullable parameter '$text'."
            else -> "Unnecessary safe call (?.) on non-nullable '$text'."
        }
    }

    private fun isSimpleName(text: String): Boolean =
        text.firstOrNull()?.let { it == '_' || it in 'a'..'z' || it in 'A'..'Z' } == true &&
            text.all { it == '_' || it in 'a'..'z' || it in 'A'..'Z' || it in '0'..'9' }
}
