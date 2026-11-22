package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol

internal object UnspecifiedImmutableFlag : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnspecifiedImmutableFlag"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnspecifiedImmutableFlag)
    }

    private val pendingIntent = classIdOf("android.app", "PendingIntent")
    private val factories = setOf("getActivity", "getActivities", "getBroadcast", "getService")
    private const val GO_FLAG_MASK = 234881024
    private const val MESSAGE = "Missing `PendingIntent` mutability flag"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (isInTestFile()) return
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId ?: return
        if (callableId.classId != pendingIntent || callableId.callableName.asString() !in factories) return
        val parameter = callee.valueParameterSymbols.getOrNull(3) ?: return
        val flags = expression.resolvedArgumentMapping?.entries
            ?.firstOrNull { it.value.symbol == parameter }?.key ?: return
        val value = PendingIntentFlagValue.read(flags) ?: return
        if (value and GO_FLAG_MASK == 0) report(unwrapArgument(flags).source, MESSAGE)
    }
}
