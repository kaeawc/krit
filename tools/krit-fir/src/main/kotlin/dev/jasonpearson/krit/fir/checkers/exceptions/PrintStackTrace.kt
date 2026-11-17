package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.insideLoggingSink
import dev.jasonpearson.krit.fir.support.isNonNothingSubtypeOf
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.isPrintToolingPath
import dev.jasonpearson.krit.fir.support.nullableClassType
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.references.FirSuperReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags `printStackTrace()` called on a Throwable, and `Thread.dumpStack()`:
 * the output goes to the console instead of a logger.
 *
 * The call counts when it resolves to the stdlib `kotlin.printStackTrace`
 * extensions (`()`, `(PrintStream)`, `(PrintWriter)`), or when the receiver it
 * is bound to is a Throwable: an extension (top-level or a member extension of
 * any class) whose receiver argument is a Throwable, or a member whose
 * containing class, or bound receiver, is a Throwable. Like Go, the body of a
 * project-declared overload does not matter, and the finding sits on the call
 * expression's first line (its receiver's). `java.lang.Thread.dumpStack()`
 * is reported on its callee name, like Go and detekt.
 *
 * A caught exception's printStackTrace() is reported in every source file,
 * like Go and detekt. A receiver only FIR proves is a Throwable (the recall
 * shapes below) is not reported in test sources, tooling paths and logging
 * sinks, which print to the console on purpose.
 *
 * An overload that takes arguments (the PrintStream / PrintWriter overloads,
 * or a project-declared one) prints wherever its arguments send the trace,
 * often into a buffer (`printStackTrace(PrintWriter(sw))` in a helper that
 * returns the trace as a string). Like Go, such a call counts only when its
 * explicit receiver is the parameter of an enclosing `catch`. A
 * `super.printStackTrace(...)` call, which delegates from an override to the
 * inherited implementation, never counts (Go never reports `super` either).
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: Go proves the receiver is a Throwable only by its spelling: the
 *   receiver must be the bare name of an enclosing `catch`'s variable, not
 *   hidden by a local or lambda parameter. For the no-argument call it misses
 *   every other Throwable receiver (a local or parameter, `e.cause`,
 *   `result.exceptionOrNull()`, a constructor call, `this`, an implicit
 *   receiver, an import alias of the stdlib function). Each of those prints
 *   the trace to the console, so it is reported.
 * - Go matches `dumpStack()` by a receiver named `Thread`. A call that
 *   resolves to `java.lang.Thread.dumpStack()` without that receiver (a
 *   static import, a Thread subclass) is reported here, and a project class
 *   named `Thread` with its own `dumpStack()` is not.
 */
internal object PrintStackTrace : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PrintStackTrace"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PrintStackTrace)
    }

    private const val MESSAGE = "Do not print a stack trace. These debug statements should be removed or replaced with a logger."
    private val PRINT_STACK_TRACE = Name.identifier("printStackTrace")
    private val STDLIB_PRINT_STACK_TRACE = CallableId(StandardClassIds.BASE_KOTLIN_PACKAGE, PRINT_STACK_TRACE)
    private val THREAD_DUMP_STACK = CallableId(classIdOf("java.lang", "Thread"), Name.identifier("dumpStack"))
    private val nullableThrowable = StandardClassIds.Throwable.nullableClassType()

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId == THREAD_DUMP_STACK) {
            report(expression.calleeReference.source ?: expression.source, MESSAGE)
            return
        }
        if (callee.name != PRINT_STACK_TRACE) return
        if (isSuperCall(expression)) return
        if (!isThrowablePrintStackTrace(expression, callee)) return
        val caught = isCaughtException(expression.explicitReceiver)
        // An overload with arguments prints wherever they send it, often into
        // a buffer (`printStackTrace(PrintWriter(sw))`). Like Go, it counts
        // only on a caught exception.
        val takesArguments = (callee as? FirFunctionSymbol<*>)?.valueParameterSymbols?.isNotEmpty() == true
        if (takesArguments && !caught) return
        // Go and detekt report a caught exception everywhere; only the shapes they miss skip console-output code.
        if (!caught && (isInTestFile() || isPrintToolingPath(containingScanPath()) || insideLoggingSink())) return
        report(expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isThrowablePrintStackTrace(expression: FirFunctionCall, callee: FirCallableSymbol<*>): Boolean {
        if (callee.callableId == STDLIB_PRINT_STACK_TRACE) return true
        // An extension, top-level or a member extension, is bound to its
        // extension receiver alone: `fun Throwable.printStackTrace(tag)`
        // declared inside any class is still called on the Throwable.
        val declaredReceiver = callee.resolvedReceiverType
        if (declaredReceiver != null) {
            return isThrowableReceiver(expression.extensionReceiver) || isThrowableType(declaredReceiver)
        }
        // A member: its containing class comes from the symbol's lookup tag,
        // which is bound to local and anonymous classes (looking one of those
        // up by class id throws).
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*>
        if (owner != null && isThrowableClass(owner)) return true
        return isThrowableReceiver(expression.dispatchReceiver)
    }

    // `super.printStackTrace(...)` inside an override delegates to the
    // inherited implementation; the call that prints is the one that calls
    // the override.
    private fun isSuperCall(expression: FirFunctionCall): Boolean =
        (expression.explicitReceiver as? FirQualifiedAccessExpression)?.calleeReference is FirSuperReference

    // Whether [receiver], through a safe call, smart casts, `!!` and casts,
    // reads the parameter of a `catch` enclosing the call.
    context(context: CheckerContext)
    private fun isCaughtException(receiver: FirExpression?): Boolean {
        var value = receiver ?: return false
        while (true) {
            value = when {
                value is FirCheckedSafeCallSubject -> value.originalReceiverRef.value
                value is FirSmartCastExpression -> value.originalExpression
                value is FirCheckNotNullCall -> value.argumentList.arguments.singleOrNull() ?: return false
                value is FirTypeOperatorCall &&
                    (value.operation == FirOperation.AS || value.operation == FirOperation.SAFE_AS) ->
                    value.argumentList.arguments.singleOrNull() ?: return false
                else -> break
            }
        }
        val access = value as? FirPropertyAccessExpression ?: return false
        if (access.explicitReceiver != null) return false
        val variable = access.calleeReference.toResolvedVariableSymbol() ?: return false
        return context.containingElements.any { it is FirCatch && it.parameter.symbol == variable }
    }

    context(context: CheckerContext)
    private fun isThrowableClass(owner: FirClassSymbol<*>): Boolean =
        isOrInheritsFrom(owner, StandardClassIds.Throwable, context.session, lookupInterfaces = false)

    context(context: CheckerContext)
    private fun isThrowableReceiver(receiver: FirExpression?): Boolean {
        val type = receiver?.let { runCatching { it.resolvedType }.getOrNull() } ?: return false
        return isThrowableType(type)
    }

    context(context: CheckerContext)
    private fun isThrowableType(type: ConeKotlinType): Boolean = type.isNonNothingSubtypeOf(nullableThrowable)
}
