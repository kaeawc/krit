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
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.InlineStatus
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSamConversionExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.expressions.unwrapArgument
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
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
// name, not the declaration. The checker walks the code the finally block
// runs in its own coroutine, and reports:
// - any call that resolves to a suspend function, qualified or not, whatever
//   its name (`job.join()`, `deferred.await()`, a project suspend function),
//   which Go misses when it is qualified or not on the list;
// - an unqualified kotlinx.coroutines `launch` / `async`, which Go reports
//   too: a child started in a cancelled scope is cancelled before its body
//   runs.
// It walks into lambdas that run in place: those passed to an inline function
// (`with`, `runCatching`), to a suspend function (`withContext`,
// `coroutineScope`, `withTimeout`, a project `retry { }`), and the body of a
// builder whose coroutine is a child of the caller's job (an unqualified
// `launch` / `async`, or a builder whose context argument may carry a Job:
// `runBlocking(ctx)`, `scope.launch(ctx)`).
// It does not report:
// - a call that only shares a name with a suspend function (a project
//   `fun delay(ms: Long)`, `Thread.join()`), which Go reports by the name;
// - `runBlocking` itself, which Go reports by the name: it is not a suspend
//   function;
// - a `withContext` / `launch` / `async` / `runBlocking` whose context makes
//   NonCancellable the Job (`NonCancellable`, `Dispatchers.IO +
//   NonCancellable`, but not `NonCancellable + job`, where the right-hand Job
//   replaces it) and everything inside it, which Go reports: that is the
//   idiomatic fix, and its body runs even when the caller is cancelled;
// - code that does not run in the finally block's coroutine, which Go reports
//   by the name: a lambda passed to a non-suspend, non-inline function (the
//   body of `scope.launch { }`, `GlobalScope.launch { }`, `flow { }`,
//   `sequence { }`, `runBlocking { }` without a Job-bearing context), a lambda
//   stored in a value, and the bodies of local functions, classes and objects;
// - a restricted suspension on kotlin.sequences.SequenceScope (`yield`), which
//   Go reports by the name: a sequence has no Job, so nothing cancels it;
// - a `try` whose code runs under an enclosing `withContext(NonCancellable)`,
//   which Go reports.
// A call inside a `try` nested in the `finally` is reported once; Go reports a
// call in a nested `finally` once per enclosing `finally` block.
internal object SuspendFunInFinallySection : FirExpressionChecker<FirTryExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "SuspendFunInFinallySection"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val tryExpressionCheckers = setOf(SuspendFunInFinallySection)
    }

    private val coroutinesPackage = FqName("kotlinx.coroutines")
    private val stdlibCoroutinesPackage = FqName("kotlin.coroutines")
    private val launch = CallableId(coroutinesPackage, Name.identifier("launch"))
    private val async = CallableId(coroutinesPackage, Name.identifier("async"))
    private val withContext = CallableId(coroutinesPackage, Name.identifier("withContext"))
    private val runBlocking = CallableId(coroutinesPackage, Name.identifier("runBlocking"))
    private val contextTakingBuilders = setOf(launch, async, withContext, runBlocking)
    private val contextParameter = Name.identifier("context")
    private val plus = Name.identifier("plus")
    private val sequenceScope = ClassId(FqName("kotlin.sequences"), Name.identifier("SequenceScope"))

    private val nonCancellable = ClassId(coroutinesPackage, Name.identifier("NonCancellable"))
    private val jobType = classType(ClassId(coroutinesPackage, Name.identifier("Job")))
    private val nonCancellableType = classType(nonCancellable)
    private val emptyCoroutineContext = ClassId(stdlibCoroutinesPackage, Name.identifier("EmptyCoroutineContext"))

    // Context elements that cannot carry a Job: adding one to a context keeps
    // the Job already there.
    private val jobFreeTypes = listOf(
        classType(ClassId(coroutinesPackage, Name.identifier("CoroutineDispatcher"))),
        classType(ClassId(coroutinesPackage, Name.identifier("CoroutineName"))),
        classType(ClassId(coroutinesPackage, Name.identifier("CoroutineExceptionHandler"))),
    )

    private fun classType(id: ClassId): ConeKotlinType = id.constructClassLikeType(emptyArray(), isMarkedNullable = false)

    // The Job a context argument puts in the new coroutine's context.
    private enum class ContextJob {
        // No Job element: the coroutine keeps the Job it would have had.
        NONE,
        // NonCancellable is the Job.
        NON_CANCELLABLE,
        // Some other Job, or a context that may hold one.
        OTHER,
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTryExpression) {
        val finallyBlock = expression.finallyBlock ?: return
        if (runsUnderNonCancellable()) return
        finallyBlock.accept(object : FirVisitorVoid() {
            // The lambdas of the calls walked so far that run in place, in the
            // finally block's coroutine (or in a child of it).
            private val inPlace = HashSet<FirAnonymousFunction>()

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
                inPlace += inPlaceLambdas(functionCall)
                functionCall.acceptChildren(this)
            }

            override fun visitImplicitInvokeCall(implicitInvokeCall: FirImplicitInvokeCall) {
                visitFunctionCall(implicitInvokeCall)
            }

            // A lambda runs in the finally block only when the call it is
            // passed to runs it there; a stored lambda runs later.
            override fun visitAnonymousFunction(anonymousFunction: FirAnonymousFunction) {
                if (anonymousFunction in inPlace) anonymousFunction.acceptChildren(this)
            }

            // Local functions, classes and objects run only when called, and
            // a call of a local suspend function is reported where it is made.
            override fun visitNamedFunction(namedFunction: FirNamedFunction) {}

            override fun visitRegularClass(regularClass: FirRegularClass) {}

            override fun visitAnonymousObject(anonymousObject: FirAnonymousObject) {}
        })
    }

    // Reports the call when it suspends in the finally block, and returns
    // whether the walk continues into its receiver, arguments and lambdas.
    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun checkCall(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return true
        val callableId = symbol.callableId
        if (callableId in contextTakingBuilders && contextJob(call) == ContextJob.NON_CANCELLABLE) return false
        val source = call.source ?: return true
        // Compiler-generated calls (a for loop's hasNext, a destructuring
        // componentN) are not written in the finally block. The selector of a
        // safe call (`job?.join()`) carries a fake kind but is the written call.
        if (source.kind is KtFakeSourceElementKind && source.kind != KtFakeSourceElementKind.DesugaredSafeCallExpression) return true
        if (isSequenceYield(call, symbol)) return true
        val reported = (symbol.isSuspend && callableId != runBlocking) ||
            ((callableId == launch || callableId == async) && call.explicitReceiver == null)
        if (reported) {
            val name = writtenName(call) ?: symbol.name.asString()
            report(source, "Suspend function '$name' called in finally block. This may not execute if the coroutine is cancelled.")
        }
        return true
    }

    // A restricted suspension on a sequence builder (`yield`, `yieldAll`, or
    // an extension on SequenceScope): a sequence has no Job to cancel.
    context(context: CheckerContext)
    private fun isSequenceYield(call: FirFunctionCall, symbol: FirNamedFunctionSymbol): Boolean {
        if (symbol.callableId.classId == sequenceScope) return true
        val receivers = listOfNotNull(call.dispatchReceiver, call.extensionReceiver)
        return receivers.any { it.resolvedType.fullyExpandedType().classId == sequenceScope }
    }

    // The lambdas passed to this call that run in the caller's coroutine, or
    // in a coroutine that is cancelled with it.
    context(context: CheckerContext)
    private fun inPlaceLambdas(call: FirFunctionCall): List<FirAnonymousFunction> {
        val lambdas = call.argumentList.arguments.mapNotNull { lambdaOf(it) }
        if (lambdas.isEmpty()) return emptyList()
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol
        val builderRunsInCaller = when (symbol?.callableId) {
            // A child of the receiver scope, which is the caller's own when
            // the call is unqualified; a context that makes NonCancellable the
            // Job stops the walk before this.
            launch, async -> call.explicitReceiver == null || contextJob(call) == ContextJob.OTHER
            // A fresh coroutine, unless its context hands it a Job to join.
            runBlocking -> contextJob(call) == ContextJob.OTHER
            else -> symbol?.isSuspend == true
        }
        return lambdas.filter { builderRunsInCaller || it.inlineStatus == InlineStatus.Inline || it.inlineStatus == InlineStatus.CrossInline }
    }

    private fun lambdaOf(argument: FirExpression): FirAnonymousFunction? {
        var unwrapped = argument.unwrapArgument()
        if (unwrapped is FirSamConversionExpression) unwrapped = unwrapped.expression.unwrapArgument()
        return (unwrapped as? FirAnonymousFunctionExpression)?.anonymousFunction
    }

    // Whether the try runs inside `withContext(<NonCancellable Job>) { }`,
    // looking out through inline lambdas and Job-free `withContext` blocks,
    // and stopping at any other lambda, function or class.
    context(context: CheckerContext)
    private fun runsUnderNonCancellable(): Boolean {
        val path = context.containingElements
        for (i in path.indices.reversed()) {
            when (val element = path[i]) {
                is FirAnonymousFunction -> {
                    if (element.inlineStatus == InlineStatus.Inline || element.inlineStatus == InlineStatus.CrossInline) continue
                    val call = owningCall(path, i, element) ?: return false
                    val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
                    if (symbol.callableId != withContext) return false
                    when (contextJob(call)) {
                        ContextJob.NON_CANCELLABLE -> return true
                        ContextJob.NONE -> continue
                        ContextJob.OTHER -> return false
                    }
                }
                is FirFunction, is FirClass -> return false
                else -> {}
            }
        }
        return false
    }

    private fun owningCall(path: List<FirElement>, index: Int, lambda: FirAnonymousFunction): FirFunctionCall? {
        for (j in index - 1 downTo 0) {
            val call = path[j] as? FirFunctionCall ?: continue
            return call.takeIf { c -> c.argumentList.arguments.any { lambdaOf(it) === lambda } }
        }
        return null
    }

    context(context: CheckerContext)
    private fun contextJob(call: FirFunctionCall): ContextJob {
        val argument = call.resolvedArgumentMapping
            ?.entries?.firstOrNull { it.value.name == contextParameter }?.key ?: return ContextJob.NONE
        return contextJob(argument)
    }

    // CoroutineContext.plus is right-biased: an element on the right replaces
    // one with the same key on the left. So the Job of a `+` chain is its
    // rightmost operand that may carry one.
    context(context: CheckerContext)
    private fun contextJob(expression: FirExpression): ContextJob {
        val operands = ArrayList<FirExpression>()
        flattenPlus(expression.unwrapArgument(), operands)
        for (operand in operands.asReversed()) {
            val type = operand.resolvedType.fullyExpandedType()
            when {
                // The value's own type: a NonCancellable reference (through an
                // import alias, a typealias, a local val, or a smart cast) has
                // the object's type. Types are only compared, never resolved
                // by class id, so a local or anonymous object is safe here.
                type.isSubtypeOf(nonCancellableType, context.session) -> return ContextJob.NON_CANCELLABLE
                isJobFree(type) -> continue
                else -> return ContextJob.OTHER
            }
        }
        return ContextJob.NONE
    }

    private fun flattenPlus(expression: FirExpression, into: MutableList<FirExpression>) {
        val call = expression as? FirFunctionCall
        val receiver = call?.explicitReceiver
        if (call == null || receiver == null || call.calleeReference.toResolvedCallableSymbol()?.name != plus) {
            into += expression
            return
        }
        flattenPlus(receiver, into)
        call.argumentList.arguments.forEach { flattenPlus(it.unwrapArgument(), into) }
    }

    context(context: CheckerContext)
    private fun isJobFree(type: ConeKotlinType): Boolean {
        if (type.classId == emptyCoroutineContext) return true
        if (type.isSubtypeOf(jobType, context.session)) return false
        return jobFreeTypes.any { type.isSubtypeOf(it, context.session) }
    }

    // The callee as written: the call name, or for an invoked suspend value
    // (`block()`), the value's name.
    private fun writtenName(call: FirFunctionCall): String? {
        if (call !is FirImplicitInvokeCall) return call.calleeReference.source?.text?.toString()
        val receiver = call.explicitReceiver as? FirPropertyAccessExpression ?: return null
        return receiver.calleeReference.source?.text?.toString()
    }
}
