package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression

/**
 * Returns the expression behind named and spread argument wrappers.
 *
 * Non-wrapped expressions are returned unchanged. Nested wrappers are removed
 * all the way down.
 */
fun unwrapArgument(argument: FirExpression): FirExpression {
    var current = argument
    while (current is FirWrappedArgumentExpression) current = current.expression
    return current
}
