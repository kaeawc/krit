package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement

internal object CursorLoopArguments {
    fun invariant(
        source: KtSourceElement,
        call: LighterASTNode,
        body: LighterASTNode,
        locals: Set<String>,
        constantCalls: Set<Int>,
    ): Boolean {
        val args = lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST }
        val argument = args?.let { lightChildren(source, it).singleOrNull { child ->
            child.tokenType == KtNodeTypes.VALUE_ARGUMENT
        } }
        val expression = argument?.let { significantChildren(source, it).singleOrNull() } ?: return false
        val name = lightText(source, expression)
        val bound = name !in locals && CursorLoopBindings.outside(source, call, body, name, column = true)
        return plainString(source, expression) || call.startOffset in constantCalls ||
            (expression.tokenType == KtNodeTypes.REFERENCE_EXPRESSION && bound)
    }

    private fun plainString(source: KtSourceElement, expression: LighterASTNode): Boolean =
        expression.tokenType == KtNodeTypes.STRING_TEMPLATE && CursorLoopSyntax.descendants(source, expression).none {
            it.tokenType in setOf(KtNodeTypes.LONG_STRING_TEMPLATE_ENTRY, KtNodeTypes.SHORT_STRING_TEMPLATE_ENTRY)
        }
}
