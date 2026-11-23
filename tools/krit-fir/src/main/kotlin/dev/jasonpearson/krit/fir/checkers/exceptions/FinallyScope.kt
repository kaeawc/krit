package dev.jasonpearson.krit.fir.checkers.exceptions

import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.InlineStatus
import org.jetbrains.kotlin.fir.expressions.FirTryExpression

/**
 * What sits between each enclosing `finally` block and the checked jump.
 *
 * For every `try` whose `finally` block contains the checked element, the list holds the containing elements
 * below that block, outermost first. [elements] is the checker's `containingElements`.
 */
internal fun finallyTails(elements: List<FirElement>): List<List<FirElement>> {
    val tails = ArrayList<List<FirElement>>()
    for ((index, element) in elements.withIndex()) {
        if (element !is FirTryExpression) continue
        val finallyBlock = element.finallyBlock ?: continue
        val at = elements.subList(index + 1, elements.size).indexOfFirst { it === finallyBlock }
        if (at >= 0) tails += elements.subList(index + 2 + at, elements.size)
    }
    return tails
}

/**
 * Whether a `throw` below a finally block leaves that block: no function, class or object that is not run in place
 * sits between them. Inline lambdas are followed; execution of any other lambda is conservatively left unknown.
 */
internal fun leavesFinally(tail: List<FirElement>): Boolean = tail.none { element ->
    when (element) {
        is FirAnonymousFunction -> element.inlineStatus != InlineStatus.Inline
        is FirFunction, is FirClass -> true
        else -> false
    }
}
