package dev.jasonpearson.krit.fir.checkers.protocol

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol

/** Test-only rule: its source file is the only registration. */
object ProtocolProbe : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ProtocolProbe"
    val checks = java.util.concurrent.atomic.AtomicInteger()
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ProtocolProbe)
    }
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        checks.incrementAndGet()
        if (expression.calleeReference.toResolvedCallableSymbol()?.name?.asString() == "protocolProbe") {
            report(expression.source, "configured: ${config()["tag"]}")
        }
    }
}
