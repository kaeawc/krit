package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement

internal object CursorLoopReceivers {
    fun invariant(source: KtSourceElement, call: LighterASTNode, body: LighterASTNode, locals: Set<String>): Boolean {
        if (!directCall(source, call, body)) return false
        val qualified = qualifiedCall(lightSourceOf(call, source)) ?: return false
        val receiver = significantChildren(source, qualified).firstOrNull() ?: return false
        if (receiver.tokenType != KtNodeTypes.REFERENCE_EXPRESSION) return false
        val name = lightText(source, receiver)
        return name !in locals && CursorLoopBindings.outside(source, call, body, name)
    }

    private fun directCall(source: KtSourceElement, call: LighterASTNode, body: LighterASTNode): Boolean {
        val parents = generateSequence(source.treeStructure.getParent(call)) { source.treeStructure.getParent(it) }
        for (scope in parents) {
            if (scope == body) return true
            if (CursorLoopBindings.executionBoundary(scope)) return false
        }
        return false
    }
}
