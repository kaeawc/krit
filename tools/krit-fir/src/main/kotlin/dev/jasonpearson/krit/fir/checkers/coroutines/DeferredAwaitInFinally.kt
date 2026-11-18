package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.nullableStarProjectedType
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `Deferred.await()` inside a `finally` block: if the awaited coroutine
// failed, await() throws, and that exception replaces the one the try block
// was propagating.
//
// Mirrors the Go DeferredAwaitInFinally rule:
// - The call sits anywhere inside a `finally` block, at any depth: nested
//   try blocks, `if`/`when` branches, lambdas, local functions, and local
//   classes do not end the search.
// - A call named runCatching around the await, inside the finally block and
//   not past a named function, exempts it, whatever that runCatching resolves
//   to (Go matches it by name).
// - The finding is on the callee name, so a chain split across lines is
//   reported on the `.await()` line.
//
// Where Go matches any call written `x.await()`, the checker requires the
// called function to await a Deferred or bridge a future into a coroutine:
// - a member `await` of kotlinx.coroutines.Deferred or a subtype;
// - an `await` extension whose receiver is a Deferred or a subtype, in any
//   package (a project's own `Deferred<T>.await(timeoutMs)`);
// - any suspend `await` extension, in any package: the future bridges of
//   kotlinx.coroutines.future / .tasks / .guava, androidx.concurrent.futures,
//   androidx.work, retrofit2, or a project's own, which rethrow the awaited
//   failure like Deferred.await();
// - any `await` declared in kotlinx.coroutines or a subpackage.
// So:
// - A member `await()` of a type that is not a Deferred (a CountDownLatch, a
//   local class, an object expression), or a blocking `await` extension on
//   one, is not reported (Go reports it).
// - An implicit-receiver `await()` (inside `with(deferred)` or a Deferred
//   extension) or an import-aliased await is reported (Go needs a `.await`
//   navigation).
// The exemptions and boundaries are also read from the resolved tree:
// - An await in the try block of a try inside the finally block whose catch
//   takes Throwable or Exception is not reported: that catch keeps the
//   failure from replacing the original exception, as runCatching does. Go
//   draws the same line by the catch's type name.
// - A runCatching only exempts an await it wraps: one in its lambda, not in
//   its receiver (`d.await().runCatching { }`), and not a runCatching outside
//   the finally block, which catches the await's exception only after it has
//   replaced the original one. Go exempts both.
// Like Go, an await in a `launch` or `async` block started in the finally
// block is reported: a scope builder in the finally block (coroutineScope,
// withContext, runBlocking, withTimeout) waits for that child and rethrows its
// failure, and a parent scope a child fails into can surface that failure in
// place of the original one, so the checker cannot prove the block is safe.
internal object DeferredAwaitInFinally : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "DeferredAwaitInFinally"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(DeferredAwaitInFinally)
    }

    private const val MESSAGE =
        "Deferred.await() in finally block can throw and mask the original exception. Wrap in runCatching."

    private val AWAIT = Name.identifier("await")
    private const val RUN_CATCHING = "runCatching"
    private const val COROUTINES_PACKAGE = "kotlinx.coroutines"
    private val catchAllClassIds = setOf(
        classIdOf("java.lang", "Throwable"),
        classIdOf("java.lang", "Exception"),
        classIdOf("kotlin", "Throwable"),
        classIdOf("kotlin", "Exception"),
    )
    private val deferredClassId = classIdOf("kotlinx.coroutines", "Deferred")
    private val nullableDeferredType = deferredClassId.nullableStarProjectedType(1)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name != AWAIT) return
        if (!isAwaitingCall(callee, context.session)) return
        if (!inFinallyBlock(expression)) return
        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }

    private fun isAwaitingCall(callee: FirCallableSymbol<*>, session: FirSession): Boolean {
        if (isCoroutinesAwait(callee.callableId?.packageName)) return true
        val receiverType = callee.resolvedReceiverType
        if (receiverType != null) {
            // An extension: a suspend await bridge, or an await on a Deferred.
            // The subtype check walks the type's supertypes; it never resolves
            // a class id through the symbol provider.
            return callee.isSuspend || receiverType.isSubtypeOf(nullableDeferredType, session)
        }
        // A member of Deferred or of a class implementing it. The owner comes
        // from the symbol's lookup tag, bound for local and anonymous classes.
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        return isDeferred(owner, session)
    }

    // An await declared in kotlinx.coroutines or a subpackage.
    private fun isCoroutinesAwait(packageName: FqName?): Boolean {
        val name = packageName?.asString() ?: return false
        return name == COROUTINES_PACKAGE || name.startsWith("$COROUTINES_PACKAGE.")
    }

    // Walks the enclosing elements from the call outward to the nearest
    // finally block that contains it.
    context(context: CheckerContext)
    private fun inFinallyBlock(expression: FirFunctionCall): Boolean {
        val path = context.containingElements
        var child: FirElement = expression
        var start = path.lastIndex
        if (path.lastOrNull() === expression) start--
        // Go's runCatching lookup stops at the nearest function_declaration.
        var runCatchingCounts = true
        for (i in start downTo 0) {
            val parent = path[i]
            when (parent) {
                is FirTryExpression -> {
                    if (parent.finallyBlock === child) return true
                    // A try in the finally block whose catch takes any Exception, and does not throw again,
                    // keeps the await's failure there.
                    if (parent.tryBlock === child && parent.catches.any { catchesAnyException(it) && !throwsAgain(it) }) {
                        return false
                    }
                }
                is FirNamedFunction -> runCatchingCounts = false
                is FirFunctionCall -> if (
                    runCatchingCounts &&
                    !isReceiverOf(parent, child) &&
                    parent.calleeReference.name.asString() == RUN_CATCHING
                ) {
                    return false
                }
                else -> {}
            }
            child = parent
        }
        return false
    }

    context(context: CheckerContext)
    private fun catchesAnyException(catch: FirCatch): Boolean =
        catch.parameter.returnTypeRef.coneType.expandedClassType()?.lookupTag?.classId in catchAllClassIds

    // A throw anywhere in the catch block (a rethrow, a wrapped failure, a selective rethrow of cancellation) lets a
    // failure leave the finally block and replace the exception in flight.
    private fun throwsAgain(catch: FirCatch): Boolean {
        var found = false
        catch.block.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirThrowExpression) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun isReceiverOf(call: FirFunctionCall, child: FirElement): Boolean =
        child === call.explicitReceiver || child === call.dispatchReceiver || child === call.extensionReceiver

    private fun isDeferred(symbol: FirClassSymbol<*>, session: FirSession): Boolean =
        isOrInheritsFrom(symbol, deferredClassId, session, lookupInterfaces = true)
}
