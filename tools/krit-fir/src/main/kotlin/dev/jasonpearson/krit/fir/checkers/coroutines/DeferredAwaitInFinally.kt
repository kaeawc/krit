package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `Deferred.await()` inside a `finally` block: if the awaited coroutine
// failed, await() throws, and that exception replaces the one the try block
// was propagating.
//
// Mirrors the Go DeferredAwaitInFinally rule:
// - The call sits anywhere inside a `finally` block, at any depth: nested
//   try/catch blocks, `if`/`when` branches, lambdas, local functions, and
//   local classes do not end the search.
// - A call named runCatching around the await, inside the finally block and
//   not past a named function, exempts it, whatever that runCatching resolves
//   to (Go matches it by name).
// - The finding is on the line where the call expression starts (its
//   receiver, for a chain split across lines).
//
// Where Go matches any call written `x.await()`, the checker requires the
// called function to be `await` declared by kotlinx.coroutines.Deferred or a
// subtype, so:
// - `await()` on a type that is not a Deferred (a CountDownLatch, a local
//   class) is not reported (Go reports it).
// - An implicit-receiver `await()` on a Deferred (inside `with(deferred)` or
//   a Deferred extension) is reported (Go needs a `.await` navigation).
// The exemptions and boundaries are also read from the resolved tree:
// - A runCatching only exempts an await it wraps: one in its lambda, not in
//   its receiver (`d.await().runCatching { }`), and not a runCatching outside
//   the finally block, which catches the await's exception only after it has
//   replaced the original one. Go exempts both.
// - An await in the block of a coroutine started by kotlinx.coroutines
//   `launch` or `async` runs outside the finally block and cannot replace
//   its exception, so it is not reported (Go reports it).
internal object DeferredAwaitInFinally : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "DeferredAwaitInFinally"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(DeferredAwaitInFinally)
    }

    private const val MESSAGE =
        "Deferred.await() in finally block can throw and mask the original exception. Wrap in runCatching."

    private val AWAIT = Name.identifier("await")
    private const val RUN_CATCHING = "runCatching"
    private val deferredClassId = ClassId.topLevel(FqName("kotlinx.coroutines.Deferred"))
    private val coroutinesPackage = FqName("kotlinx.coroutines")
    private val coroutineStarters = setOf(
        CallableId(coroutinesPackage, Name.identifier("launch")),
        CallableId(coroutinesPackage, Name.identifier("async")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name != AWAIT) return
        // A member of Deferred or of a class implementing it. The owner comes
        // from the symbol's lookup tag, bound for local and anonymous classes.
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return
        if (!isDeferred(owner, context.session)) return
        if (!inFinallyBlock(expression)) return
        report(expression.source, MESSAGE)
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
        // The lambdas passed on the way out.
        val lambdas = mutableSetOf<FirAnonymousFunction>()
        for (i in start downTo 0) {
            val parent = path[i]
            when (parent) {
                is FirTryExpression -> if (parent.finallyBlock === child) return true
                is FirNamedFunction -> runCatchingCounts = false
                is FirAnonymousFunction -> lambdas += parent
                is FirFunctionCall -> if (!isReceiverOf(parent, child)) {
                    val symbol = parent.calleeReference.toResolvedCallableSymbol()
                    if (symbol?.callableId in coroutineStarters && passesLambda(parent, lambdas)) return false
                    if (runCatchingCounts && parent.calleeReference.name.asString() == RUN_CATCHING) return false
                }
                else -> {}
            }
            child = parent
        }
        return false
    }

    // Whether one of [call]'s arguments is a lambda in [lambdas].
    private fun passesLambda(call: FirFunctionCall, lambdas: Set<FirAnonymousFunction>): Boolean =
        call.arguments.any { (it as? FirAnonymousFunctionExpression)?.anonymousFunction in lambdas }

    private fun isReceiverOf(call: FirFunctionCall, child: FirElement): Boolean =
        child === call.explicitReceiver || child === call.dispatchReceiver || child === call.extensionReceiver

    private fun isDeferred(symbol: FirClassSymbol<*>, session: FirSession): Boolean =
        symbol.classId == deferredClassId ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                .any { it.classId == deferredClassId }
}
