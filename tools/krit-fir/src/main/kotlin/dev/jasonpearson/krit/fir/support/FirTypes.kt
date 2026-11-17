package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
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
