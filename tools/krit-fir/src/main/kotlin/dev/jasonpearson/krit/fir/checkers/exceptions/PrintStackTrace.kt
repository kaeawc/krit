package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags `printStackTrace()` called on a Throwable: the output goes to the
 * console instead of a logger.
 *
 * The call counts when it resolves to the stdlib `kotlin.printStackTrace`
 * extensions (`()`, `(PrintStream)`, `(PrintWriter)`), or when the receiver it
 * is bound to is a Throwable: an extension (top-level or a member extension of
 * any class) whose receiver argument is a Throwable, or a member whose
 * containing class, or bound receiver, is a Throwable. Like Go, the arguments
 * and the body of a project-declared overload do not matter, and the finding
 * sits on the call expression's first line (its receiver's). Go also skips
 * `.gradle.kts` scripts, matched here on the scan's own spelling of the path.
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
 *   Throwable (a shadowing local, `this.e`, `other.e`) is not a Throwable's
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
        if (!isThrowablePrintStackTrace(expression, callee)) return
        if (containingScanPath()?.endsWith(".gradle.kts") == true) return
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

    context(context: CheckerContext)
    private fun isThrowableClass(owner: FirClassSymbol<*>): Boolean =
        owner.classId == StandardClassIds.Throwable ||
            lookupSuperTypes(owner, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == StandardClassIds.Throwable }

    context(context: CheckerContext)
    private fun isThrowableReceiver(receiver: FirExpression?): Boolean {
        val type = receiver?.let { runCatching { it.resolvedType }.getOrNull() } ?: return false
        return isThrowableType(type)
    }

    context(context: CheckerContext)
    private fun isThrowableType(type: ConeKotlinType): Boolean {
        if (type.isNothingOrNullableNothing) return false
        return type.isSubtypeOf(nullableThrowable, context.session)
    }
}
