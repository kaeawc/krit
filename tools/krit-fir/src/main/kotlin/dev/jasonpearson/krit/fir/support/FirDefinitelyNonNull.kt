package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.UnresolvedExpressionTypeAccess
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirLocalPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.canBeNull

/**
 * Non-null evidence only: platform/error types and nullable generic bounds are not proof.
 * Only local values and parameters use smart casts; other property casts may rely on a source-only module boundary.
 */
@OptIn(UnresolvedExpressionTypeAccess::class)
context(context: CheckerContext)
fun FirExpression.isDefinitelyNonNullOperand(): Boolean {
    val operand = nonNullEvidenceOperand()
    val type = operand.coneTypeOrNull ?: return false
    return type !is ConeFlexibleType && !operand.hasErrorType() && !type.canBeNull(context.session)
}

private fun FirExpression.nonNullEvidenceOperand(): FirExpression {
    var operand = this
    while (operand is FirSmartCastExpression) {
        if (operand.isStable && operand.originalExpression.isLocalValue()) return operand
        operand = operand.originalExpression
    }
    return operand
}

private fun FirExpression.isLocalValue(): Boolean {
    val symbol = (this as? FirPropertyAccessExpression)?.calleeReference?.toResolvedVariableSymbol()
    return symbol is FirValueParameterSymbol || symbol is FirLocalPropertySymbol
}
