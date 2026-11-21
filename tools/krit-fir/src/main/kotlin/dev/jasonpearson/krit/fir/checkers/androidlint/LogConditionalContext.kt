package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unwrapLightParens
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.impl.FirElseIfTrueCondition
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression

private enum class ConditionalStatus {
    CONTINUE,
    CONDITIONAL,
    UNCONDITIONAL,
}

context(context: CheckerContext)
internal fun isConditional(call: FirFunctionCall): Boolean {
    var conditional = false
    for (element in context.containingElements.asReversed()) {
        when (conditionalStatus(element, call)) {
            ConditionalStatus.CONTINUE -> Unit
            ConditionalStatus.CONDITIONAL -> {
                conditional = true
                break
            }
            ConditionalStatus.UNCONDITIONAL -> break
        }
    }
    return conditional
}

context(context: CheckerContext)
private fun conditionalStatus(element: Any, call: FirFunctionCall): ConditionalStatus = when (element) {
    is FirFunctionCall -> if (blocksConditionalSearch(element, call)) {
        ConditionalStatus.UNCONDITIONAL
    } else {
        ConditionalStatus.CONTINUE
    }
    is FirNamedFunction -> if (element.isLocal) ConditionalStatus.CONTINUE else ConditionalStatus.UNCONDITIONAL
    is FirProperty -> if (element.isLocal) ConditionalStatus.CONTINUE else ConditionalStatus.UNCONDITIONAL
    is FirRegularClass, is FirAnonymousObject, is FirConstructor, is FirPropertyAccessor,
    is FirAnonymousInitializer, is FirFile -> ConditionalStatus.UNCONDITIONAL
    is FirWhenExpression -> whenExpressionStatus(element, call)
    else -> ConditionalStatus.CONTINUE
}

private fun blocksConditionalSearch(element: FirFunctionCall, call: FirFunctionCall): Boolean {
    if (element === call) return false
    val source = element.source ?: return false
    if (selectorCallExpression(source) == null) return false
    return !receiverContainsCall(element, call)
}

private fun receiverContainsCall(element: FirFunctionCall, call: FirFunctionCall): Boolean {
    val receiver = element.explicitReceiver?.source ?: return false
    val source = call.source ?: return false
    return receiver.startOffset <= source.startOffset && source.endOffset <= receiver.endOffset
}

context(context: CheckerContext)
private fun whenExpressionStatus(element: FirWhenExpression, call: FirFunctionCall): ConditionalStatus {
    val elements = context.containingElements
    val holding = element.branches.indexOfFirst { branch ->
        elements.any { it === branch } || sourceContainsCall(branch.condition.source, call)
    }
    if (holding < 0) {
        return if (element.source?.elementType == KtNodeTypes.IF) {
            ConditionalStatus.CONDITIONAL
        } else {
            ConditionalStatus.CONTINUE
        }
    }
    val branch = element.branches[holding]
    return if (element.source?.elementType == KtNodeTypes.IF) {
        ifBranchStatus(element, holding)
    } else {
        whenBranchStatus(branch.condition, branch.source)
    }
}

private fun sourceContainsCall(source: KtSourceElement?, call: FirFunctionCall): Boolean {
    val condition = source ?: return false
    val callSource = call.source ?: return false
    return condition.startOffset <= callSource.startOffset && callSource.endOffset <= condition.endOffset
}

private fun ifBranchStatus(element: FirWhenExpression, holding: Int): ConditionalStatus {
    val branch = element.branches[holding]
    val condition = if (branch.condition is FirElseIfTrueCondition) {
        element.branches.getOrNull(holding - 1)?.condition
    } else {
        branch.condition
    }
    return if (condition?.let { !isLiteralTrue(it) } == true) {
        ConditionalStatus.CONDITIONAL
    } else {
        ConditionalStatus.UNCONDITIONAL
    }
}

private fun whenBranchStatus(condition: FirExpression, source: KtSourceElement?): ConditionalStatus {
    if (condition is FirElseIfTrueCondition) return ConditionalStatus.UNCONDITIONAL
    if (source == null) return conditionStatus(!isLiteralTrue(condition))
    val conditions = lightChildren(source, source.lighterASTNode).filter {
        it.tokenType in setOf(
            KtNodeTypes.WHEN_CONDITION_EXPRESSION,
            KtNodeTypes.WHEN_CONDITION_IN_RANGE,
            KtNodeTypes.WHEN_CONDITION_IS_PATTERN,
        )
    }
    return conditionStatus(conditions.any { conditionNode ->
        if (conditionNode.tokenType != KtNodeTypes.WHEN_CONDITION_EXPRESSION) {
            true
        } else {
            val value = significantChildren(source, conditionNode).firstOrNull()
            value != null && lightText(source, unwrapLightParens(source, value)).trim() != "true"
        }
    })
}

private fun conditionStatus(isConditional: Boolean): ConditionalStatus =
    if (isConditional) ConditionalStatus.CONDITIONAL else ConditionalStatus.UNCONDITIONAL

private fun isLiteralTrue(expression: FirExpression): Boolean {
    val literal = expression.unwrapSmartcastExpression() as? FirLiteralExpression ?: return false
    return literal.value == true
}
