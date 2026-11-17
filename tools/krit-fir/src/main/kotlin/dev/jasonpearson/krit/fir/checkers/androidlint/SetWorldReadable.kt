package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.name.Name

/**
 * Flags `file.setReadable(true, false)` on a `java.io.File`, which makes the file readable by everyone, as Android
 * Lint's SetWorldReadable does. See [reportWorldPermission] for what counts as the call and its two arguments.
 *
 * Like the Go rule, each argument may be a literal, a `!` of one, or a `val` of the same file initialized with one.
 * Go types the receiver from declarations it can read in the file; this checker reads the resolved callee, which
 * also covers an alias, a call result, `!!` and a smart cast (SetWorldReadableDivergence).
 */
internal object SetWorldReadable : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SetWorldReadable"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SetWorldReadable)
    }

    private const val MESSAGE = "Setting file permissions to world-readable can be risky, review carefully"
    private val method = Name.identifier("setReadable")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        reportWorldPermission(expression, method, MESSAGE)
    }
}
