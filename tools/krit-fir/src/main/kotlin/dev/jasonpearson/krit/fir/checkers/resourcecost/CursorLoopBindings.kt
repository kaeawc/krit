package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

internal object CursorLoopBindings {
    fun outside(
        source: KtSourceElement,
        call: LighterASTNode,
        body: LighterASTNode,
        name: String,
        column: Boolean = false,
    ): Boolean {
        val parents = generateSequence(source.treeStructure.getParent(call)) { source.treeStructure.getParent(it) }
        var proven = false
        for (scope in parents.filter { it.startOffset < body.startOffset }) {
            if (implicitParameter(source, scope, name)) break
            val declaration = lightChildren(source, scope).firstOrNull {
                it.startOffset < body.startOffset &&
                    name in declarationNames(source, it)
            }
            if (declaration != null) {
                val visible = bindingVisible(source, scope, declaration, body, column)
                proven = visible && !written(source, scope, name)
                break
            }
            if (executionBoundary(scope)) break
        }
        return proven
    }

    private fun bindingVisible(
        source: KtSourceElement,
        scope: LighterASTNode,
        declaration: LighterASTNode,
        body: LighterASTNode,
        column: Boolean,
    ): Boolean {
        val mutable = column && lightChildren(source, declaration).any { it.tokenType == KtTokens.VAR_KEYWORD }
        val lambdaParameter = !column && declaration.tokenType == KtNodeTypes.VALUE_PARAMETER_LIST &&
            CursorLoopLambdas.direct(source, scope, body)
        return !mutable && (lambdaParameter || visible(source, scope, declaration, body))
    }

    fun executionBoundary(node: LighterASTNode): Boolean = node.tokenType in setOf(
        KtNodeTypes.FUN, KtNodeTypes.FUNCTION_LITERAL, KtNodeTypes.LAMBDA_EXPRESSION,
        KtNodeTypes.CLASS, KtNodeTypes.OBJECT_DECLARATION, KtNodeTypes.CLASS_BODY,
        KtNodeTypes.SECONDARY_CONSTRUCTOR, KtNodeTypes.PROPERTY_ACCESSOR,
    )

    private fun implicitParameter(source: KtSourceElement, scope: LighterASTNode, name: String): Boolean =
        name == "it" && scope.tokenType == KtNodeTypes.FUNCTION_LITERAL &&
            lightChildren(source, scope).none { it.tokenType == KtNodeTypes.VALUE_PARAMETER_LIST }

    private fun visible(
        source: KtSourceElement,
        scope: LighterASTNode,
        child: LighterASTNode,
        body: LighterASTNode,
    ): Boolean = when (child.tokenType) {
        KtNodeTypes.VALUE_PARAMETER_LIST -> scope.tokenType == KtNodeTypes.FUN
        KtNodeTypes.PROPERTY, KtNodeTypes.DESTRUCTURING_DECLARATION -> {
            val local = scope.tokenType == KtNodeTypes.BLOCK && child.endOffset <= body.startOffset
            val delegated = lightChildren(source, child).any { it.tokenType == KtNodeTypes.PROPERTY_DELEGATE }
            local && !delegated
        }
        else -> false
    }

    private fun declarationNames(source: KtSourceElement, node: LighterASTNode): Set<String> = buildSet {
        for (child in lightChildren(source, node)) {
            if (child.tokenType == KtTokens.IDENTIFIER) add(lightText(source, child))
            if (child.tokenType in setOf(KtNodeTypes.DESTRUCTURING_DECLARATION_ENTRY,
                    KtNodeTypes.DESTRUCTURING_DECLARATION, KtNodeTypes.VALUE_PARAMETER,
                    KtNodeTypes.VALUE_PARAMETER_LIST)) {
                addAll(declarationNames(source, child))
            }
        }
    }

    private fun written(source: KtSourceElement, scope: LighterASTNode, name: String): Boolean =
        CursorLoopSyntax.descendants(source, scope).any { node ->
            val children = significantChildren(source, node)
            val operation = children.firstOrNull { it.tokenType == KtNodeTypes.OPERATION_REFERENCE }
            val writes = operation?.let { lightText(source, it) } in
                setOf("=", "+=", "-=", "*=", "/=", "%=", "++", "--")
            val target = children.firstOrNull { it.tokenType != KtNodeTypes.OPERATION_REFERENCE }
            writes && target != null && CursorLoopSyntax.descendants(source, target).any {
                it.tokenType == KtTokens.IDENTIFIER && lightText(source, it) == name
            }
        }
}
