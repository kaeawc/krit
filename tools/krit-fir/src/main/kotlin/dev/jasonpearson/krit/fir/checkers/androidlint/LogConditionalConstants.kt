package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapLightParens
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.fir.FirEvaluatorResult
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirExpressionEvaluator
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.PrivateConstantEvaluatorAPI
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol

internal fun isBinary(expression: FirExpression): Boolean = expression.source?.let {
    unwrapLightParens(it, it.lighterASTNode).tokenType == KtNodeTypes.BINARY_EXPRESSION
} == true

@OptIn(PrivateConstantEvaluatorAPI::class)
context(context: CheckerContext)
internal fun isConstant(argument: FirExpression, localValues: Boolean, seen: MutableSet<FirPropertySymbol>): Boolean {
    val expression = unwrapArgument(argument).unwrapSmartcastExpression()
    if (hasFoldedConstant(expression)) return true
    return when (expression) {
        is FirLiteralExpression -> true
        is FirStringConcatenationCall -> isConstantStringConcatenation(expression, seen)
        is FirFunctionCall -> isConstantFunctionCall(expression, seen)
        is FirQualifiedAccessExpression -> isConstantPropertyAccess(expression, localValues, seen)
        else -> false
    }
}

@OptIn(PrivateConstantEvaluatorAPI::class)
context(context: CheckerContext)
private fun hasFoldedConstant(expression: FirExpression): Boolean {
    if (!isBinary(expression)) return false
    val evaluation = FirExpressionEvaluator.evaluateExpression(expression, context.session)
    val folded = (evaluation as? FirEvaluatorResult.Evaluated)?.result
    return folded is FirLiteralExpression
}

context(context: CheckerContext)
private fun isConstantStringConcatenation(
    expression: FirStringConcatenationCall,
    seen: MutableSet<FirPropertySymbol>,
): Boolean = expression.argumentList.arguments.all { isConstant(it, isBinary(expression), seen) }

context(context: CheckerContext)
private fun isConstantFunctionCall(expression: FirFunctionCall, seen: MutableSet<FirPropertySymbol>): Boolean {
    if (!isBinary(expression) || !isArithmeticCall(expression)) return false
    val receiverIsConstant = expression.explicitReceiver?.let { isConstant(it, true, seen) } == true
    return receiverIsConstant && expression.argumentList.arguments.all { isConstant(it, true, seen) }
}

private fun isArithmeticCall(expression: FirFunctionCall): Boolean {
    val operatorName = expression.calleeReference.toResolvedCallableSymbol()?.name?.asString()
    return operatorName in setOf("plus", "minus", "times", "div", "rem")
}

context(context: CheckerContext)
private fun isConstantPropertyAccess(
    expression: FirQualifiedAccessExpression,
    localValues: Boolean,
    seen: MutableSet<FirPropertySymbol>,
): Boolean {
    val symbol = expression.calleeReference.toResolvedCallableSymbol()
    if (symbol is FirFieldSymbol) return symbol.hasConstantInitializer
    val property = symbol as? FirPropertySymbol ?: return false
    if (!seen.add(property)) {
        seen.remove(property)
        return false
    }
    return try {
        isConstantProperty(property, localValues, seen)
    } finally {
        seen.remove(property)
    }
}

context(context: CheckerContext)
private fun isConstantProperty(
    property: FirPropertySymbol,
    localValues: Boolean,
    seen: MutableSet<FirPropertySymbol>,
): Boolean = when {
    property.isConst -> true
    localValues && property.isLocal && property.isVal && !property.hasDelegate ->
        property.initializerInCheckedFile()?.let { isConstant(it, true, seen) } == true
    else -> false
}
