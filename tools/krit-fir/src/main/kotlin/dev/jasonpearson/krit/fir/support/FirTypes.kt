package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.UnresolvedExpressionTypeAccess
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible

/** This type with every type alias expanded and a platform type reduced to its lower bound. */
context(context: CheckerContext)
fun ConeKotlinType.expandedBound(): ConeKotlinType = fullyExpandedType().lowerBoundIfFlexible()

/**
 * The class type this type names once aliases are expanded and a platform type is reduced to its lower bound.
 *
 * Null for a type parameter, a definitely-not-null type, or any other type that is not a class type.
 */
context(context: CheckerContext)
fun ConeKotlinType.expandedClassType(): ConeClassLikeType? = expandedBound() as? ConeClassLikeType

@OptIn(UnresolvedExpressionTypeAccess::class)
fun FirExpression.hasErrorType(): Boolean = coneTypeOrNull?.containsErrorType() == true

private fun ConeKotlinType.containsErrorType(): Boolean = when (this) {
    is ConeErrorType -> true
    is ConeFlexibleType -> lowerBound.containsErrorType() || upperBound.containsErrorType()
    else -> typeArguments.any { (it as? ConeKotlinTypeProjection)?.type?.containsErrorType() == true }
}
