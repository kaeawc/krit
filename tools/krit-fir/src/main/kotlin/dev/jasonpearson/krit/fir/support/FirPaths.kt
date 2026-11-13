package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

/**
 * The elements from the outermost declaration around [target] down to [target], or null when the walk misses it.
 *
 * The checker context lists enclosing declarations, not the expressions between them and the call. This walks the
 * outermost non-class declaration to recover those parents, so call it only after cheaper checks have passed.
 */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
fun pathTo(target: FirElement): List<FirElement>? {
    val rootSymbol = context.containingDeclarations
        .firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol } ?: return null
    val stack = ArrayList<FirElement>()
    var found: List<FirElement>? = null
    rootSymbol.fir.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (found != null) return
            stack += element
            if (element === target) {
                found = ArrayList(stack)
            } else {
                element.acceptChildren(this)
            }
            stack.removeAt(stack.lastIndex)
        }
    })
    return found
}
