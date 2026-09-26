package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.text

// Flags `xs.filter { pred }.first()` (and firstOrNull, last, lastOrNull,
// single, singleOrNull, count, any, none) that can be `xs.first { pred }`.
//
// Like the Go rule:
// - the terminal call is one of those names with no arguments at all;
// - its receiver is a `filter` call with a trailing lambda (`filter({ })`
//   and `filter(::pred)` do not count), directly or through a
//   safe call (`xs?.filter { }?.first()`), with an explicit or an implicit
//   receiver;
// - the message quotes the lambda as written (without a label) and the
//   finding sits on the first line of the whole call chain.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Precision: `filter` must be the stdlib filter (kotlin.collections,
//   kotlin.sequences, kotlin.text) or kotlinx.coroutines.flow.filter, and the
//   terminal the stdlib (or, after a Flow filter, the Flow) function of that
//   name, since only those have the predicate overload the message suggests.
//   Go matches the names, so it reports a custom `filter` (a query builder, a
//   Java Stream), a same-package `first()` extension, and a Flow
//   `last()`/`single()` (Flow has no predicate overload of those) whenever it
//   cannot resolve the receiver.
// - Precision: a terminal that already takes a lambda
//   (`xs.filter { a }.first { b }`) is not reported. Go counts only the
//   parenthesized arguments and suggests `.first { a }`, dropping `b`.
// - Recall: the receiver's type is proven by resolution. Go skips a receiver
//   whose name it resolves to a type outside List/MutableList/Collection/
//   Iterable/Set/MutableSet/Sequence/Map/MutableMap (an ArrayList, an Array, a
//   String, a Flow, a subclass of List), although the stdlib (or Flow)
//   predicate overload exists for each of them.
// - Recall: a parenthesized filter call `(xs.filter { }).first()` is still the
//   filter's result; Go needs the call as the direct receiver. Go also misses
//   `xs.filter() { }.first()`, which tree-sitter nests in a second call, and
//   import aliases of filter or the terminal, since it compares call names.
internal object UnnecessaryFilter : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessaryFilter"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnnecessaryFilter)
    }

    private val stdlibPackages = setOf(
        FqName("kotlin.collections"),
        FqName("kotlin.sequences"),
        FqName("kotlin.text"),
    )
    private val flowPackage = FqName("kotlinx.coroutines.flow")

    private val stdlibTerminators = setOf(
        "first", "firstOrNull", "last", "lastOrNull", "single", "singleOrNull", "count", "any", "none",
    )

    // kotlinx.coroutines.flow has predicate overloads of these terminals only.
    private val flowTerminators = setOf("first", "firstOrNull", "count")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val terminal = stdlibFunction(expression) ?: return
        val name = terminal.callableName.asString()
        if (name !in stdlibTerminators) return
        if (expression.argumentList.arguments.isNotEmpty()) return

        val filterCall = filterCallOf(expression.explicitReceiver) ?: return
        val filter = stdlibFunction(filterCall) ?: return
        if (filter.callableName.asString() != "filter") return
        val terminalOk = when (filter.packageName) {
            in stdlibPackages -> terminal.packageName in stdlibPackages
            flowPackage -> terminal.packageName == flowPackage && name in flowTerminators
            else -> false
        }
        if (!terminalOk) return

        val lambda = filterCall.argumentList.arguments.singleOrNull() as? FirAnonymousFunctionExpression ?: return
        if (!lambda.isTrailingLambda) return
        val predicate = lambdaText(lambda) ?: return

        report(expression.source, "Replace '.filter $predicate.$name()' with '.$name $predicate'.")
    }

    // The callable id of a top-level function (stdlib and Flow operators are
    // top-level extensions), or null for a member or an unresolved call.
    private fun stdlibFunction(call: FirFunctionCall): CallableId? {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return null
        val id = symbol.callableId
        if (id.classId != null) return null
        if (id.packageName !in stdlibPackages && id.packageName != flowPackage) return null
        return id
    }

    private fun filterCallOf(receiver: FirExpression?): FirFunctionCall? = when (receiver) {
        is FirFunctionCall -> receiver
        is FirCheckedSafeCallSubject -> filterCallOf(receiver.originalReceiverRef.value)
        is FirSafeCallExpression -> receiver.selector as? FirFunctionCall
        else -> null
    }

    // The lambda literal as written, `{ ... }`, without a label or annotation
    // in front of it, the text Go quotes.
    private fun lambdaText(lambda: FirAnonymousFunctionExpression): String? {
        val text = (lambda.anonymousFunction.source ?: lambda.source)?.text?.toString() ?: return null
        val brace = text.indexOf('{')
        return if (brace < 0) null else text.substring(brace)
    }
}
