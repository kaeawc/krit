package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isNonNothingSubtypeOf
import dev.jasonpearson.krit.fir.support.nullableClassType
import dev.jasonpearson.krit.fir.support.walkWithParents
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirWhileLoop
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.resolvedType

internal object CursorLoopSymbols {
    private val cursorType = classIdOf("android.database", "Cursor").nullableClassType()
    private val stringId = classIdOf("kotlin", "String")
    private val intId = classIdOf("kotlin", "Int")

    context(context: CheckerContext)
    fun isLookup(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (symbol.receiverParameterSymbol != null) return false
        return symbol.callableId?.callableName?.asString() in CursorLoopSyntax.lookupNames &&
            symbol.valueParameterSymbols.singleOrNull()?.resolvedReturnType?.classId == stringId &&
            symbol.resolvedReturnType.classId == intId && hasCursorReceiver(call)
    }

    context(context: CheckerContext)
    private fun hasCursorReceiver(call: FirFunctionCall): Boolean =
        call.dispatchReceiver?.resolvedType?.isNonNothingSubtypeOf(cursorType) == true

    context(context: CheckerContext)
    fun isIteration(loop: FirWhileLoop): Boolean {
        var proven = false
        walkWithParents(loop.condition) { element, _ ->
            if (element is FirFunctionCall && isMoveToNext(element)) proven = true
        }
        return proven
    }

    context(context: CheckerContext)
    private fun isMoveToNext(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (symbol.receiverParameterSymbol != null) return false
        return symbol.callableId?.callableName?.asString() == "moveToNext" &&
            symbol.valueParameterSymbols.isEmpty() && hasCursorReceiver(call)
    }
}
