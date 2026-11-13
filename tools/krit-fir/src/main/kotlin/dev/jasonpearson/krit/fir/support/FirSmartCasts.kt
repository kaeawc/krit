package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression

/**
 * Returns the expression the user wrote behind compiler-inserted receiver wrappers.
 *
 * Strips smart casts and the checked subject of a safe call, repeatedly and in any order. With
 * [throughNotNull] it also strips `!!`. Anything else is returned unchanged.
 */
fun unwrapSmartCasts(expression: FirExpression, throughNotNull: Boolean = false): FirExpression {
    var current = expression
    while (true) {
        current = when {
            current is FirSmartCastExpression -> current.originalExpression
            current is FirCheckedSafeCallSubject -> current.originalReceiverRef.value
            throughNotNull && current is FirCheckNotNullCall ->
                current.argumentList.arguments.firstOrNull() ?: return current
            else -> return current
        }
    }
}

/** Same as [unwrapSmartCasts] for an expression that may be absent. */
@JvmName("unwrapSmartCastsOrNull")
fun unwrapSmartCasts(expression: FirExpression?, throughNotNull: Boolean = false): FirExpression? =
    expression?.let { unwrapSmartCasts(it, throughNotNull) }
