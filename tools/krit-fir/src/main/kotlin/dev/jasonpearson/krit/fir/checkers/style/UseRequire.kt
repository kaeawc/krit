package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isDirectControlBody
import dev.jasonpearson.krit.fir.support.throwsPlainException
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression

/**
 * Port of the Go UseRequire rule: a `throw` of a new `IllegalArgumentException` with no argument or one `String`
 * message that is the whole body of an `if` branch or a loop, which `require()` replaces.
 *
 * The exception class comes from the resolved constructor, so the check follows an import alias, a type alias or a
 * qualified `java.lang.IllegalArgumentException`, and ignores a project class or function that merely shares the name
 * (Go matches the written callee name only).
 */
internal object UseRequire : FirExpressionChecker<FirThrowExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseRequire"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val throwExpressionCheckers = setOf(UseRequire)
    }

    private val illegalArgument = setOf(
        classIdOf("java.lang", "IllegalArgumentException"),
        classIdOf("kotlin", "IllegalArgumentException"),
    )

    private const val MESSAGE = "Use require() instead of throwing an IllegalArgumentException."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirThrowExpression) {
        if (!throwsPlainException(expression, illegalArgument)) return
        val source = expression.source ?: return
        if (!isDirectControlBody(source)) return
        report(source, MESSAGE)
    }
}
