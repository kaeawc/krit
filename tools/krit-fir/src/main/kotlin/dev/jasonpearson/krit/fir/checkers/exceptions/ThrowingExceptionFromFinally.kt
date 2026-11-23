package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression

// Flags a `throw` inside a `finally` block that runs as part of the block: it replaces the try's pending
// exception.
//
// Mirrors the Go ThrowingExceptionFromFinally rule. Go walks the finally block and skips a throw inside a
// nested function, anonymous function, object or class, but treats every lambda as part of the block. This
// checker skips a throw inside a non-inline lambda (`thread { throw ... }`, `lazy { ... }`),
// because in-place execution is not proven, and keeps a throw inside an inline lambda
// (`list.forEach { throw ... }`) or an inline anonymous-function literal
// (`list.forEach(fun(_) { throw ... })`). The latter intentionally differs from Go's anonymous-function
// boundary because FIR proves that the inline call runs it in place.
//
// Same as Go: the throw may be nested at any depth, and a throw whose exception a nested `try` catches is
// still reported. Each throw is reported once, even when it sits in a finally block nested in another.
internal object ThrowingExceptionFromFinally :
    FirExpressionChecker<FirThrowExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "ThrowingExceptionFromFinally"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val throwExpressionCheckers = setOf(ThrowingExceptionFromFinally)
    }

    private const val MESSAGE =
        "Do not throw an exception within a finally statement. This can discard exceptions and is confusing."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirThrowExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (finallyTails(context.containingElements).none(::leavesFinally)) return
        report(source, MESSAGE)
    }
}
