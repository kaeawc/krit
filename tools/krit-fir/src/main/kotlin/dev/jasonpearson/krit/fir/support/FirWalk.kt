package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

/** Visits [root] and its descendants with their enclosing elements, outermost first. */
fun walkWithParents(root: FirElement, visit: (FirElement, List<FirElement>) -> Unit) {
    val parents = ArrayList<FirElement>()
    root.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            visit(element, parents)
            parents += element
            element.acceptChildren(this)
            parents.removeAt(parents.lastIndex)
        }
    })
}

/** Walks only the file currently being checked, never bodies from another compilation slice. */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
fun walkCheckedFileWithParents(visit: (FirElement, List<FirElement>) -> Unit) {
    val file = context.containingFileSymbol?.fir ?: return
    walkWithParents(file, visit)
}
