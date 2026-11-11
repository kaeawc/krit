package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * True when this expression is the `null` literal.
 *
 * Argument wrappers and parentheses are not looked through, so callers that
 * need that must unwrap first.
 */
fun isNullLiteral(expression: FirExpression): Boolean =
    expression is FirLiteralExpression && expression.kind == ConstantValueKind.Null
