package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf

/**
 * Whether this type is a subtype of [supertype] and is not `Nothing` or `Nothing?`.
 *
 * `Nothing` is a subtype of every type, so a `TODO()`, `throw` or `return` expression would otherwise match any
 * supertype test although it never carries a value of that type.
 */
context(context: CheckerContext)
fun ConeKotlinType.isNonNothingSubtypeOf(supertype: ConeKotlinType): Boolean =
    !isNothingOrNullableNothing && isSubtypeOf(supertype, context.session)
