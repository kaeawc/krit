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
 * Flags `file.setWritable(true, false)` on a `java.io.File`, which makes the file writable by everyone, as Android
 * Lint's SetWorldWritable does. See [reportWorldPermission] for what counts as the call and its two arguments.
 *
 * Like the Go rule, each argument may be a literal, a `!` of one, or a `val` of the same file initialized with one.
 * Go types the receiver from declarations it can read in the file; this checker reads the resolved callee, which
 * also covers an alias, a call result, `!!` and a smart cast (SetWorldWritableDivergence). As Android Lint does and Go does
 * not, it also folds `&&` and `||` of such values and reports a File subclass that overrides the method.
 */
internal object SetWorldWritable : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SetWorldWritable"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SetWorldWritable)
    }

    private const val MESSAGE = "Setting file permissions to world-writable can be risky, review carefully"
    private val method = Name.identifier("setWritable")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        reportWorldPermission(expression, method, MESSAGE)
    }
}
