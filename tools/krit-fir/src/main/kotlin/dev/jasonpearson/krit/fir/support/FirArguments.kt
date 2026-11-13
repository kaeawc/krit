package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
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

/**
 * The call that takes [lambda] as an argument: the nearest call before [lambdaIndex] in the checker's
 * containing [elements] (outermost first) that lists it, or null when none does.
 */
fun lambdaOwner(
    elements: List<FirElement>,
    lambdaIndex: Int,
    lambda: FirAnonymousFunction,
): FirFunctionCall? {
    for (index in lambdaIndex - 1 downTo 0) {
        val call = elements[index] as? FirFunctionCall ?: continue
        val takesLambda = call.argumentList.arguments.any {
            (unwrapArgument(it) as? FirAnonymousFunctionExpression)?.anonymousFunction === lambda
        }
        if (takesLambda) return call
    }
    return null
}

/**
 * The name [argument] was written with (`enabled` in `f(enabled = false)`), or null for a positional argument.
 *
 * A resolved call drops the named-argument wrapper, so this reads the name from the argument's source.
 */
fun writtenArgumentName(argument: FirExpression): String? {
    val source = argument.source ?: return null
    val node = source.lighterASTNode
    val valueArgument = if (node.tokenType == KtNodeTypes.VALUE_ARGUMENT) node else source.treeStructure.getParent(node)
    if (valueArgument == null || valueArgument.tokenType != KtNodeTypes.VALUE_ARGUMENT) return null
    val name = lightChildren(source, valueArgument).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME } ?: return null
    return lightText(source, name).trim()
}
