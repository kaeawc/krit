package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isOnlyStatementInLambda
import dev.jasonpearson.krit.fir.support.throwsPlainException
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression

/**
 * Port of the Go UseCheckOrError rule: a `throw` of a new `IllegalStateException` with no argument or one `String`
 * message, which `check()` or `error()` replaces. The throw is reported with Go's message, except when it is the
 * only statement of a lambda.
 *
 * The exception class comes from the resolved constructor, so the check follows an import alias, a type alias or a
 * qualified `java.lang.IllegalStateException`, and ignores a project class or function that merely shares the name
 * (Go matches the written callee name only).
 */
internal object UseCheckOrError : FirExpressionChecker<FirThrowExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseCheckOrError"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val throwExpressionCheckers = setOf(UseCheckOrError)
    }

    private val illegalState = setOf(
        classIdOf("java.lang", "IllegalStateException"),
        classIdOf("kotlin", "IllegalStateException"),
    )

    private const val MESSAGE = "Use check() or error() instead of throwing an IllegalStateException."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirThrowExpression) {
        if (!throwsPlainException(expression, illegalState)) return
        val source = expression.source ?: return
        if (isOnlyStatementInLambda(source)) return
        report(source, MESSAGE)
    }
}
