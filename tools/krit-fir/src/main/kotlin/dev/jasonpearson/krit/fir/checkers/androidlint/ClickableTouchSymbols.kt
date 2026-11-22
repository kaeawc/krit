package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedBound
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.nestedClassIdOf
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.StandardClassIds

internal object ClickableTouchSymbols {
    private val view = classIdOf("android.view", "View")
    private val event = classIdOf("android.view", "MotionEvent")
    private val listener = nestedClassIdOf("android.view", "View.OnTouchListener")

    context(context: CheckerContext)
    fun viewOverride(symbol: FirNamedFunctionSymbol): Boolean {
        val parameters = if (symbol.name.asString() == "onTouchEvent") listOf(event) else emptyList()
        return symbol.rawStatus.isOverride && member(symbol, view, parameters)
    }

    context(context: CheckerContext)
    fun touchOverride(symbol: FirNamedFunctionSymbol): Boolean =
        symbol.name.asString() == "onTouch" && symbol.rawStatus.isOverride &&
            member(symbol, listener, listOf(view, event))

    context(context: CheckerContext)
    fun setter(symbol: FirNamedFunctionSymbol): Boolean =
        symbol.name.asString() == "setOnTouchListener" && member(symbol, view, listOf(listener), StandardClassIds.Unit)

    context(context: CheckerContext)
    private fun member(
        symbol: FirNamedFunctionSymbol,
        target: ClassId,
        parameters: List<ClassId>,
        result: ClassId = StandardClassIds.Boolean,
    ): Boolean {
        val enclosing = symbol.getContainingClassSymbol() ?: return false
        return symbol.receiverParameterSymbol == null &&
            symbol.resolvedReturnType.expandedBound().classId == result &&
            symbol.valueParameterSymbols.map { it.resolvedReturnType.expandedBound().classId } == parameters &&
            isOrInheritsFrom(enclosing, target, context.session, lookupInterfaces = true)
    }
}
