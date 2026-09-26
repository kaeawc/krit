package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

// Flags a suspend function called in a `finally` block: once the coroutine is
// cancelled, the first suspension point in `finally` throws
// CancellationException, so the cleanup it guards never runs. The fix is
// `withContext(NonCancellable) { ... }`.
//
// Mirrors the Go SuspendFunInFinallySection rule, which walks every call inside
// a `finally` block (lambdas, local functions and nested `try` included) and
// reports a call whose text starts with a name from a fixed list of well-known
// suspend functions and coroutine builders (`delay(`, `withContext(`,
// `launch {`, ...). That text match only sees unqualified calls, and reads the
// name, not the declaration. The checker walks the same subtree and reports:
// - any call that resolves to a suspend function, qualified or not, whatever
//   its name (`job.join()`, `deferred.await()`, a project suspend function),
//   which Go misses when it is qualified or not on the list;
// - an unqualified kotlinx.coroutines `launch` / `async`, which Go reports
//   too: a child started in a cancelled scope is cancelled before its body
//   runs.
// It does not report:
// - a call that only shares a name with a suspend function (a project
//   `fun delay(ms: Long)`), which Go reports by the name;
// - `runBlocking { }` and everything inside it, which Go reports by the name:
//   runBlocking is not a suspend function and starts a fresh coroutine that is
//   not cancelled with the caller, so its body does run;
// - a `withContext` / `launch` / `async` whose context holds `NonCancellable`
//   and everything inside it, which Go reports: that is the idiomatic fix, and
//   its body runs even when the caller is cancelled.
// A call inside a `try` nested in the `finally` is reported once; Go reports a
// call in a nested `finally` once per enclosing `finally` block.
internal object SuspendFunInFinallySection : FirExpressionChecker<FirTryExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "SuspendFunInFinallySection"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val tryExpressionCheckers = setOf(SuspendFunInFinallySection)
    }

    private val coroutinesPackage = FqName("kotlinx.coroutines")
    private val launch = CallableId(coroutinesPackage, Name.identifier("launch"))
    private val async = CallableId(coroutinesPackage, Name.identifier("async"))
    private val withContext = CallableId(coroutinesPackage, Name.identifier("withContext"))
    private val runBlocking = CallableId(coroutinesPackage, Name.identifier("runBlocking"))
    private val nonCancellable = ClassId(coroutinesPackage, Name.identifier("NonCancellable"))
    private val contextTakingBuilders = setOf(launch, async, withContext)
    private val contextParameter = Name.identifier("context")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTryExpression) {
        val finallyBlock = expression.finallyBlock ?: return
        finallyBlock.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                element.acceptChildren(this)
            }

            // A nested try's own finally block is checked when the checker
            // visits that try, so a call there is reported once.
            override fun visitTryExpression(tryExpression: FirTryExpression) {
                tryExpression.tryBlock.accept(this)
                tryExpression.catches.forEach { it.accept(this) }
            }

            override fun visitFunctionCall(functionCall: FirFunctionCall) {
                if (!checkCall(functionCall)) return
                functionCall.acceptChildren(this)
            }

            override fun visitImplicitInvokeCall(implicitInvokeCall: FirImplicitInvokeCall) {
                visitFunctionCall(implicitInvokeCall)
            }
        })
    }

    // Reports the call when it suspends in the finally block, and returns
    // whether the walk continues into its receiver, arguments and lambdas.
    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun checkCall(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return true
        val callableId = symbol.callableId
        if (callableId == runBlocking) return false
        if (callableId in contextTakingBuilders && holdsNonCancellable(call)) return false
        val source = call.source ?: return true
        if (source.kind is KtFakeSourceElementKind) return true
        val reported = symbol.isSuspend ||
            ((callableId == launch || callableId == async) && call.explicitReceiver == null)
        if (reported) {
            val name = writtenName(call) ?: symbol.name.asString()
            report(source, "Suspend function '$name' called in finally block. This may not execute if the coroutine is cancelled.")
        }
        return true
    }

    // The builder's `context` argument is NonCancellable, or a `+` chain that
    // contains it.
    private fun holdsNonCancellable(call: FirFunctionCall): Boolean {
        val argument = call.resolvedArgumentMapping
            ?.entries?.firstOrNull { it.value.name == contextParameter }?.key ?: return false
        return containsNonCancellable(argument)
    }

    private fun containsNonCancellable(expression: FirExpression): Boolean {
        // The value's own type: a NonCancellable reference (through an import
        // alias, a typealias, a local val, or a smart cast) has the object's
        // type. The class id is only compared, never resolved, so a local
        // object is safe here.
        if (expression.resolvedType.classId == nonCancellable) return true
        val unwrapped = if (expression is FirSmartCastExpression) expression.originalExpression else expression
        val plus = unwrapped as? FirFunctionCall ?: return false
        if (plus.calleeReference.toResolvedCallableSymbol()?.name?.asString() != "plus") return false
        val operands = listOfNotNull(plus.explicitReceiver) + plus.argumentList.arguments
        return operands.any { containsNonCancellable(it) }
    }

    // The callee as written: the call name, or for an invoked suspend value
    // (`block()`), the value's name.
    private fun writtenName(call: FirFunctionCall): String? {
        if (call !is FirImplicitInvokeCall) return call.calleeReference.source?.text?.toString()
        val receiver = call.explicitReceiver as? FirPropertyAccessExpression ?: return null
        return receiver.calleeReference.source?.text?.toString()
    }
}
