package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

internal object CursorLoopExemptions {
    private val annotationNames = setOf("Suppress", "SuppressWarnings", "SuppressLint")
    private val ids = setOf(
        "CursorLoopWithColumnIndexInLoop", "all", "All", "ALL",
        "resource-cost.CursorLoopWithColumnIndexInLoop", "resource-cost:CursorLoopWithColumnIndexInLoop",
    )

    fun suppressed(source: KtSourceElement, call: LighterASTNode): Boolean =
        generateSequence(call) { source.treeStructure.getParent(it) }.any { ancestor ->
            lightChildren(source, ancestor).any { annotations ->
                isAnnotationContainer(annotations) && hasSuppression(source, annotations)
            }
        }

    private fun isAnnotationContainer(node: LighterASTNode): Boolean =
        node.tokenType == KtNodeTypes.MODIFIER_LIST || node.tokenType == KtNodeTypes.FILE_ANNOTATION_LIST ||
            node.tokenType == KtNodeTypes.ANNOTATION_ENTRY

    private fun hasSuppression(source: KtSourceElement, annotations: LighterASTNode): Boolean =
        CursorLoopSyntax.descendants(source, annotations).any { annotation ->
            annotation.tokenType == KtNodeTypes.ANNOTATION_ENTRY &&
                isSuppressAnnotation(source, annotation) && suppressedArgument(source, annotation)
        }

    private fun isSuppressAnnotation(source: KtSourceElement, annotation: LighterASTNode): Boolean =
        CursorLoopSyntax.descendants(source, annotation).takeWhile {
            it.tokenType != KtNodeTypes.VALUE_ARGUMENT_LIST
        }.any { it.tokenType == KtTokens.IDENTIFIER && lightText(source, it) in annotationNames }

    private fun suppressedArgument(source: KtSourceElement, annotation: LighterASTNode): Boolean =
        CursorLoopSyntax.descendants(source, annotation).any {
            val literal = lightText(source, it).removeSurrounding("\"\"\"").removeSurrounding("\"").trim()
            val value = literal.removePrefix("detekt:").removePrefix("detekt.")
            it.tokenType == KtNodeTypes.STRING_TEMPLATE && value in ids
        }
}
