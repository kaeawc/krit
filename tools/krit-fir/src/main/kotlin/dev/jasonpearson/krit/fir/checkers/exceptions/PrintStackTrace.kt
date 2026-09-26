package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags `printStackTrace()` on a Throwable: the stack trace goes to stderr
 * instead of a logger.
 *
 * The call counts when it resolves to the stdlib `kotlin.printStackTrace`
 * extensions (`()`, `(PrintStream)`, `(PrintWriter)`) or to a
 * `printStackTrace` member of a Throwable type (the JDK's
 * `java.lang.Throwable` members, or an override in a subclass). Like Go, the
 * arguments do not matter and the finding sits on the call expression's first
 * line (its receiver's). Go also skips `.gradle.kts` scripts, matched here on
 * the scan's own spelling of the path.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: Go proves the receiver is a Throwable only by its spelling: the
 *   receiver's last identifier must equal the variable of the nearest
 *   enclosing `catch`. It misses every other Throwable receiver (a local or
 *   parameter, `e.cause`, `result.exceptionOrNull()`, a constructor call, an
 *   outer catch's variable read inside a nested catch, `this` / `super`,
 *   an implicit receiver, an import alias of the stdlib function). Each of
 *   those is a printStackTrace() call on a Throwable, so it is reported.
 * - Precision: a receiver spelled like the caught variable that is not the
 *   Throwable (a shadowing local, `this.e`, `other.e`), or a same-named
 *   extension declared outside the stdlib, is not a Throwable's
 *   printStackTrace(), so it is not reported.
 */
internal object PrintStackTrace : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PrintStackTrace"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PrintStackTrace)
    }

    private const val MESSAGE = "Use a logger instead of printStackTrace()."
    private val PRINT_STACK_TRACE = Name.identifier("printStackTrace")
    private val STDLIB_PRINT_STACK_TRACE = CallableId(StandardClassIds.BASE_KOTLIN_PACKAGE, PRINT_STACK_TRACE)
    private val nullableThrowable = StandardClassIds.Throwable.constructClassLikeType(emptyArray(), isMarkedNullable = true)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name != PRINT_STACK_TRACE) return
        if (!isThrowablePrintStackTrace(callee)) return
        if (containingScanPath()?.endsWith(".gradle.kts") == true) return
        report(expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isThrowablePrintStackTrace(callee: FirCallableSymbol<*>): Boolean {
        if (callee.callableId == STDLIB_PRINT_STACK_TRACE) return true
        // A member: its owner must be a Throwable. The owner type comes from
        // the symbol itself, so a member of a local class needs no class-id
        // lookup.
        val owner = callee.dispatchReceiverType ?: return false
        if (owner.isNothingOrNullableNothing) return false
        return owner.isSubtypeOf(nullableThrowable, context.session)
    }
}
