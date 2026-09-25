package dev.jasonpearson.krit.fir.checkers.security

import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

/**
 * The values assigned to each local variable anywhere in the outermost
 * declaration that encloses the element [context] is checking: a function, a
 * property initializer, or an init block, including every lambda, local
 * function and local class inside it. A local var's whole scope lies inside
 * that declaration, so every value it can hold after its initializer is in
 * this map. A plain assignment contributes its right-hand side; a compound
 * one (`x += y`) contributes the operand it combines with the current value
 * (`y`), since the current value is the var itself. Null when no declaration
 * encloses the element, so a local var's values are unknown.
 */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
internal fun assignedLocalValues(): Map<FirBasedSymbol<*>, List<FirExpression>>? {
    val root = context.containingDeclarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
        ?: return null
    val assigned = HashMap<FirBasedSymbol<*>, MutableList<FirExpression>>()
    root.fir.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (element is FirVariableAssignment) {
                val lValue = element.lValue
                val compound = lValue is FirDesugaredAssignmentValueReferenceExpression
                val target = (if (compound) lValue.expressionRef.value else lValue) as? FirQualifiedAccessExpression
                val symbol = target?.calleeReference?.toResolvedCallableSymbol()
                if (symbol != null) {
                    val value = element.rValue
                    val values = if (compound && value is FirFunctionCall) operands(value) else listOf(value)
                    assigned.getOrPut(symbol) { ArrayList() } += values
                }
            }
            element.acceptChildren(this)
        }
    })
    return assigned
}

// The operands `x += y` combines with the current value: the operator call's
// arguments (none for `x++`).
private fun operands(call: FirFunctionCall): List<FirExpression> =
    call.argumentList.arguments.flatMap { if (it is FirVarargArgumentsExpression) it.arguments else listOf(it) }

/** A local `var`: its later assignments decide its value along with its initializer. */
internal fun isLocalVar(symbol: FirPropertySymbol): Boolean = symbol.isLocal && !symbol.isVal

/**
 * True when every value assigned to the local var [symbol] after its
 * initializer passes [hardcoded]; false when its values are unknown
 * ([assigned] is null).
 */
internal fun reassignmentsAll(
    symbol: FirPropertySymbol,
    assigned: Map<FirBasedSymbol<*>, List<FirExpression>>?,
    hardcoded: (FirExpression) -> Boolean,
): Boolean {
    if (assigned == null) return false
    return assigned[symbol].orEmpty().all(hardcoded)
}
