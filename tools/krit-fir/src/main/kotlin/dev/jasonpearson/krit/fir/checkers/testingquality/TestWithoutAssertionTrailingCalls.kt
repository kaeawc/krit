package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import dev.jasonpearson.krit.fir.support.lambdaArgumentOf
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol

internal fun trailingResultCheck(last: FirElement?, depth: Int): Boolean = when (last) {
    is FirTypeOperatorCall -> last.operation == FirOperation.AS
    is FirCheckNotNullCall -> true
    is FirProperty -> trailingResultCheck(last.initializer, depth)
    is FirFunctionCall -> trailingCallCheck(last, depth)
    else -> false
}

private fun trailingCallCheck(call: FirFunctionCall, depth: Int): Boolean {
    val id = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return true
    val name = id.callableName.asString()
    if (name in GoTestAssertionNames.throwingCheckCalls) return true
    if (depth >= MAX_TRAILING_CHECK_DEPTH || name !in GoTestAssertionNames.runnerCalls) return false
    val lambda = call.arguments.lastOrNull()?.let(::unwrapArgument)?.let(::lambdaArgumentOf) ?: return false
    return lambda.anonymousFunction.body?.let { TestWithoutAssertionCalls.trailingCheck(it, depth + 1) } == true
}

private const val MAX_TRAILING_CHECK_DEPTH = 3
