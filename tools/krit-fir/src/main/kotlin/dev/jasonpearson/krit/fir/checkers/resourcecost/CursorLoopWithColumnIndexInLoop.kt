package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.pathTo
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirWhileLoop

internal object CursorLoopWithColumnIndexInLoop : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CursorLoopWithColumnIndexInLoop"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(CursorLoopWithColumnIndexInLoop)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (!CursorLoopSymbols.isLookup(expression)) return
        val call = selectorCallExpression(source) ?: return
        if (CursorLoopExemptions.suppressed(source, call)) return
        val name = CursorLoopSyntax.callName(source, call) ?: return
        val anchor = CursorLoopSyntax.callee(source, call) ?: return
        val receiverText = CursorLoopSyntax.receiverText(source, call)
        val resolvedLoops = pathTo(expression).orEmpty().filterIsInstance<FirWhileLoop>()
        for (loop in CursorLoopSyntax.enclosingLoops(source, call)) {
            val loopSource = lightSourceOf(loop, source)
            val resolved = resolvedLoops.firstOrNull { it.source?.startOffset == loopSource.startOffset }
            if (resolved != null &&
                CursorLoopSyntax.firstLookup(source, loop, CursorLoopConstants.calls(resolved)) == call &&
                CursorLoopSymbols.isIteration(resolved)) {
                report(lightSourceOf(anchor, source),
                    "$receiverText.$name() inside while loop; hoist column index lookup before the loop.")
            }
        }
    }
}
