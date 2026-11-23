package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirReturnExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression

// Flags a `return` inside a `finally` block that leaves the block: the try's pending exception is dropped.
//
// Mirrors the Go ReturnFromFinally rule. Go walks the finally block and skips a return inside a nested
// function, object or class, and a `return@name` whose label equals the name of the lambda's callee. This
// checker follows the return's resolved target instead: a return is reported when the function it returns
// from is declared outside the finally block, so a `return@outer` that targets a custom label of a lambda
// inside the block is not reported, and a `return@name` that targets the enclosing function is.
//
// Same as Go: a return in an inline lambda that returns from the enclosing function is reported, and
// `ignoreLabeled` skips every return written with a label. Each return is reported once, even when it sits
// in a finally block nested in another.
internal object ReturnFromFinally : FirReturnExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ReturnFromFinally"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val returnExpressionCheckers = setOf(ReturnFromFinally)
    }

    private const val MESSAGE = "Do not return within a finally statement. This can discard exceptions."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirReturnExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (config()["ignoreLabeled"] == true && lightText(source, source.lighterASTNode).startsWith("return@")) return
        val target = expression.target.labeledElement
        if (finallyTails(context.containingElements).none { tail -> tail.none { it === target } }) return
        report(source, MESSAGE)
    }
}
