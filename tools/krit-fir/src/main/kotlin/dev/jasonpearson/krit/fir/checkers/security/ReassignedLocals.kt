package dev.jasonpearson.krit.fir.checkers.security

import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

/**
 * The local variables assigned anywhere in the outermost declaration that
 * encloses the element [context] is checking: a function, a property
 * initializer, or an init block, including every lambda, local function and
 * local class inside it. A local var's whole scope lies inside that
 * declaration, so a var not in the set is never reassigned after its
 * initializer (compound assignments such as `+=` and `++` count). Null when
 * no declaration encloses the element, so no local var can be proven
 * unassigned.
 */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
internal fun assignedLocals(): Set<FirBasedSymbol<*>>? {
    val root = context.containingDeclarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
        ?: return null
    val assigned = HashSet<FirBasedSymbol<*>>()
    root.fir.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (element is FirVariableAssignment) {
                // A compound assignment (`+=`, `++`) wraps its target.
                val lValue = element.lValue
                val target = (if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue)
                    as? FirQualifiedAccessExpression
                target?.calleeReference?.toResolvedCallableSymbol()?.let { assigned += it }
            }
            element.acceptChildren(this)
        }
    })
    return assigned
}

/**
 * True when [symbol] is a local var that [assigned] (from [assignedLocals])
 * does not prove unassigned: its value may not be its initializer.
 */
internal fun isReassignedLocalVar(symbol: FirPropertySymbol, assigned: () -> Set<FirBasedSymbol<*>>?): Boolean {
    if (!symbol.isLocal || symbol.isVal) return false
    val set = assigned() ?: return true
    return symbol in set
}
