package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapLightParens
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirEvaluatorResult
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirExpressionEvaluator
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.PrivateConstantEvaluatorAPI
import org.jetbrains.kotlin.fir.expressions.impl.FirElseIfTrueCondition
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol

object LogConditional : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LogConditional"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LogConditional)
    }

    private val logClassId = classIdOf("android.util", "Log")
    private val levelNames = setOf("v", "d", "i", "println")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callableId = expression.calleeReference.toResolvedCallableSymbol()?.callableId ?: return
        val name = callableId.callableName.asString()
        if (callableId.classId != logClassId || name !in levelNames || isInTestFile()) return
        val message = expression.argumentList.arguments.getOrNull(if (name == "println") 2 else 1) ?: return
        if (!messageDoesWork(message) || isConditional(expression)) return
        report(
            expression.source,
            "The log call Log.$name(...) should be conditional: surround with `if (Log.isLoggable(...))` or " +
                "`if (BuildConfig.DEBUG) { ... }`",
        )
    }

    context(context: CheckerContext)
    private fun messageDoesWork(argument: FirExpression): Boolean {
        val expression = unwrapArgument(argument).unwrapSmartcastExpression()
        if (expression is FirLiteralExpression) return false
        if (expression is FirStringConcatenationCall || isBinary(expression)) return !isConstant(expression, false, mutableSetOf())
        if (expression is FirQualifiedAccessExpression && expression !is FirFunctionCall) {
            val source = expression.source ?: return false
            if (unwrapLightParens(source, source.lighterASTNode).tokenType == KtNodeTypes.REFERENCE_EXPRESSION) return false
            val symbol = expression.calleeReference.toResolvedCallableSymbol()
            return symbol !is FirFieldSymbol && (symbol !is FirPropertySymbol || !symbol.isConst)
        }
        return true
    }

    private fun isBinary(expression: FirExpression): Boolean = expression.source?.let {
        unwrapLightParens(it, it.lighterASTNode).tokenType == KtNodeTypes.BINARY_EXPRESSION
    } == true

    @OptIn(PrivateConstantEvaluatorAPI::class)
    context(context: CheckerContext)
    private fun isConstant(argument: FirExpression, localValues: Boolean, seen: MutableSet<FirPropertySymbol>): Boolean {
        val expression = unwrapArgument(argument).unwrapSmartcastExpression()
        if (isBinary(expression)) {
            val folded = (FirExpressionEvaluator.evaluateExpression(expression, context.session) as? FirEvaluatorResult.Evaluated)?.result
            if (folded is FirLiteralExpression) return true
        }
        return when (expression) {
            is FirLiteralExpression -> true
            is FirStringConcatenationCall -> expression.argumentList.arguments.all { isConstant(it, isBinary(expression), seen) }
            is FirFunctionCall -> isBinary(expression) && expression.calleeReference.toResolvedCallableSymbol()?.name?.asString() in
                setOf("plus", "minus", "times", "div", "rem") &&
                expression.explicitReceiver?.let { isConstant(it, true, seen) } == true &&
                expression.argumentList.arguments.all { isConstant(it, true, seen) }
            is FirQualifiedAccessExpression -> {
                val symbol = expression.calleeReference.toResolvedCallableSymbol()
                if (symbol is FirFieldSymbol) return symbol.hasConstantInitializer
                val property = symbol as? FirPropertySymbol
                when {
                    property == null || !seen.add(property) -> false
                    property.isConst -> true
                    localValues && property.isLocal && property.isVal && !property.hasDelegate ->
                        property.initializerInCheckedFile()?.let { isConstant(it, true, seen) } == true
                    else -> false
                }.also { if (property != null) seen.remove(property) }
            }
            else -> false
        }
    }

    context(context: CheckerContext)
    private fun isConditional(call: FirFunctionCall): Boolean {
        val elements = context.containingElements
        for (index in elements.indices.reversed()) {
            when (val element = elements[index]) {
                is FirFunctionCall -> if (element !== call && element.source?.let {
                    selectorCallExpression(it)
                } != null) return false
                is FirNamedFunction -> if (!element.isLocal) return false
                is FirProperty -> if (!element.isLocal) return false
                is FirRegularClass, is FirAnonymousObject, is FirConstructor, is FirPropertyAccessor,
                is FirAnonymousInitializer, is FirFile -> return false
                is FirWhenExpression -> {
                    val holding = element.branches.indexOfFirst { branch ->
                        elements.any { it === branch } || branch.condition.source?.let { condition ->
                            call.source?.let { source ->
                                condition.startOffset <= source.startOffset && source.endOffset <= condition.endOffset
                            }
                        } == true
                    }
                    if (holding < 0) {
                        if (element.source?.elementType == KtNodeTypes.IF) return true
                        continue
                    }
                    val branch = element.branches[holding]
                    if (element.source?.elementType == KtNodeTypes.IF) {
                        val condition = if (branch.condition is FirElseIfTrueCondition) {
                            element.branches.getOrNull(holding - 1)?.condition
                        } else branch.condition
                        return condition?.let { !isLiteralTrue(it) } ?: false
                    }
                    if (branch.condition is FirElseIfTrueCondition) return false
                    val source = branch.source ?: return !isLiteralTrue(branch.condition)
                    val conditions = lightChildren(source, source.lighterASTNode).filter {
                        it.tokenType in setOf(KtNodeTypes.WHEN_CONDITION_EXPRESSION, KtNodeTypes.WHEN_CONDITION_IN_RANGE, KtNodeTypes.WHEN_CONDITION_IS_PATTERN)
                    }
                    return conditions.any { condition ->
                        if (condition.tokenType != KtNodeTypes.WHEN_CONDITION_EXPRESSION) return@any true
                        val value = significantChildren(source, condition).firstOrNull() ?: return@any false
                        lightText(source, unwrapLightParens(source, value)).trim() != "true"
                    }
                }
                else -> Unit
            }
        }
        return false
    }

    private fun isLiteralTrue(expression: FirExpression): Boolean {
        val literal = expression.unwrapSmartcastExpression() as? FirLiteralExpression ?: return false
        return literal.value == true
    }
}
