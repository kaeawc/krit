package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapLightParens
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
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
        if (expression is FirStringConcatenationCall || isBinary(expression)) {
            return !isConstant(expression, false, mutableSetOf(), mutableMapOf())
        }
        if (expression is FirQualifiedAccessExpression && expression !is FirFunctionCall) {
            val source = expression.source ?: return false
            if (
                unwrapLightParens(source, source.lighterASTNode).tokenType == KtNodeTypes.REFERENCE_EXPRESSION
            ) return false
            val symbol = expression.calleeReference.toResolvedCallableSymbol()
            return symbol !is FirFieldSymbol && (symbol !is FirPropertySymbol || !symbol.isConst)
        }
        return true
    }
}
