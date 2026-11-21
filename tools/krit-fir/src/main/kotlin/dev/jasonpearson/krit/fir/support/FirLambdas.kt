package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionTypeConversionExpression

/**
 * Returns [argument] as a lambda written after the call's parentheses, or null for anything else.
 *
 * A SAM conversion around the lambda (`filter { }` on a Java functional interface) is looked through. Named and
 * spread wrappers are not, so callers that need that must unwrap first.
 */
fun trailingLambdaOf(argument: FirExpression): FirAnonymousFunctionExpression? {
    return lambdaArgumentOf(argument)?.takeIf { it.isTrailingLambda }
}

fun lambdaArgumentOf(argument: FirExpression): FirAnonymousFunctionExpression? {
    val inner = if (argument is FirFunctionTypeConversionExpression) argument.expression else argument
    return inner as? FirAnonymousFunctionExpression
}
