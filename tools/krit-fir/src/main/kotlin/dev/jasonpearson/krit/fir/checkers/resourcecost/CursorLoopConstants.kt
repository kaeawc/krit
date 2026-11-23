package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.checkers.constantStringValue
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.walkWithParents
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirWhileLoop
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol

internal object CursorLoopConstants {
    context(context: CheckerContext)
    fun calls(loop: FirWhileLoop): Set<Int> = buildSet {
        walkWithParents(loop.block) { element, _ ->
            if (element is FirFunctionCall && constantArgument(element)) {
                element.source?.let { selectorCallExpression(it) }?.let { add(it.startOffset) }
            }
        }
    }

    context(context: CheckerContext)
    private fun constantArgument(call: FirFunctionCall): Boolean {
        val argument = call.argumentList.arguments.singleOrNull()?.let { unwrapArgument(it) }
        val access = argument as? FirPropertyAccessExpression ?: return false
        if (access.explicitReceiver != null && access.explicitReceiver !is FirResolvedQualifier) return false
        return when (val symbol = access.calleeReference.toResolvedCallableSymbol()) {
            is FirPropertySymbol -> symbol.isConst && symbol.constantStringValue(context.session) != null
            is FirFieldSymbol -> symbol.isVal && symbol.resolvedStatus.isStatic &&
                symbol.hasConstantInitializer && symbol.constantStringValue(context.session) != null
            else -> false
        }
    }
}
