package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

internal object CursorLoopLocals {
    fun names(source: KtSourceElement, body: LighterASTNode): Set<String> = buildSet {
        for (node in CursorLoopSyntax.descendants(source, body)) {
            when (node.tokenType) {
                KtNodeTypes.PROPERTY, KtNodeTypes.VALUE_PARAMETER, KtNodeTypes.DESTRUCTURING_DECLARATION_ENTRY,
                KtNodeTypes.FUN, KtNodeTypes.CLASS, KtNodeTypes.OBJECT_DECLARATION -> {
                    val identifier = lightChildren(source, node).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
                    if (identifier != null) add(lightText(source, identifier))
                }
                KtNodeTypes.FUNCTION_LITERAL -> {
                    val parameters = lightChildren(source, node).firstOrNull {
                        it.tokenType == KtNodeTypes.VALUE_PARAMETER_LIST
                    }
                    if (parameters == null) add("it")
                }
                else -> Unit
            }
        }
    }
}
