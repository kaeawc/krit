package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement

internal object CursorLoopLambdas {
    fun direct(source: KtSourceElement, scope: LighterASTNode, body: LighterASTNode): Boolean {
        if (scope.tokenType != KtNodeTypes.FUNCTION_LITERAL) return false
        val loop = source.treeStructure.getParent(body) ?: return false
        if (loop.tokenType != KtNodeTypes.WHILE) return false
        val block = parentSkippingLabelsAndAnnotations(source, loop) ?: return false
        if (block.tokenType != KtNodeTypes.BLOCK || source.treeStructure.getParent(block) != scope) return false
        val lambda = source.treeStructure.getParent(scope) ?: return false
        return callArgument(source, lambda)
    }

    private fun callArgument(source: KtSourceElement, lambda: LighterASTNode): Boolean {
        val trailingArgument = parentSkippingLabelsAndAnnotations(source, lambda) ?: return false
        return when (trailingArgument.tokenType) {
            KtNodeTypes.LAMBDA_ARGUMENT ->
                source.treeStructure.getParent(trailingArgument)?.tokenType == KtNodeTypes.CALL_EXPRESSION
            KtNodeTypes.VALUE_ARGUMENT -> {
                val argument = source.treeStructure.getParent(lambda) ?: return false
                if (argument.tokenType != KtNodeTypes.VALUE_ARGUMENT) return false
                val arguments = source.treeStructure.getParent(argument) ?: return false
                val values = lightChildren(source, arguments).filter { it.tokenType == KtNodeTypes.VALUE_ARGUMENT }
                arguments.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST && values.size == 1 &&
                    source.treeStructure.getParent(arguments)?.tokenType == KtNodeTypes.CALL_EXPRESSION
            }
            else -> false
        }
    }

    private fun parentSkippingLabelsAndAnnotations(
        source: KtSourceElement,
        node: LighterASTNode,
    ): LighterASTNode? {
        var parent = source.treeStructure.getParent(node) ?: return null
        while (parent.tokenType == KtNodeTypes.LABELED_EXPRESSION ||
            parent.tokenType == KtNodeTypes.ANNOTATED_EXPRESSION
        ) {
            parent = source.treeStructure.getParent(parent) ?: return null
        }
        return parent
    }
}
