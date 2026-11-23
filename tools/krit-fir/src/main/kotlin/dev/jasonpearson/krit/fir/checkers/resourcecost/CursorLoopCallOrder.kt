package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement

internal object CursorLoopCallOrder {
    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    fun descendants(source: KtSourceElement, node: LighterASTNode): Sequence<LighterASTNode> = sequence {
        yield(node)
        val children = lightChildren(source, node)
        val selector = significantChildren(source, node).lastOrNull()
        if (node.tokenType in qualifiedTypes && selector?.tokenType == KtNodeTypes.CALL_EXPRESSION) {
            yield(selector)
            for (child in children.filterNot { it == selector }) yieldAll(descendants(source, child))
            for (child in lightChildren(source, selector)) yieldAll(descendants(source, child))
        } else {
            for (child in children) yieldAll(descendants(source, child))
        }
    }
}
