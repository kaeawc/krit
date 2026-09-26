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
import org.jetbrains.kotlin.fir.expressions.FirSamConversionExpression
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
// - both calls are made by those names: an import alias
//   (`import kotlin.collections.first as head`) is not reported, since K2
//   drops the aliased name from the default imports, so the suggested
//   `.first { }` would not resolve;
// - the message quotes the lambda as written (including an explicit label) and the
//   finding sits on the first line of the whole call chain.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Precision: the terminal must be the stdlib function of that name
//   (kotlin.collections, kotlin.sequences, kotlin.text), or, after
//   kotlinx.coroutines.flow.filter, the Flow one, since only those have the
//   predicate overload the message suggests. The filter must be the stdlib
//   (or Flow) filter. A custom filter can change the result type or semantics
//   even when its receiver is an Iterable, Sequence, or CharSequence.
//   Go matches the names, so it reports a custom `filter` on another type
//   (a query builder, a Java Stream), a same-package `first()` extension,
//   and a Flow `last()`/`single()` (Flow has no predicate overload of those)
//   whenever it cannot resolve the receiver.
// - Precision: a terminal that already takes a lambda
//   (`xs.filter { a }.first { b }`) is not reported. Go counts only the
//   parenthesized arguments and suggests `.first { a }`, dropping `b`.
// - Recall: the receiver's type is proven by resolution. Go skips a receiver
//   whose name it resolves to a type outside List/MutableList/Collection/
//   Iterable/Set/MutableSet/Sequence/Map/MutableMap (an ArrayList, an Array, a
//   String, a Flow, a subclass of List, a user Iterable), although the stdlib
//   (or Flow) predicate overload exists for each of them.
// - Recall: a parenthesized filter call `(xs.filter { }).first()` is still the
//   filter's result; Go needs the call as the direct receiver. Go also misses
//   `xs.filter() { }.first()`, which tree-sitter nests in a second call.
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
        val terminal = resolvedFunction(expression) ?: return
        if (!terminal.isTopLevelIn(stdlibPackages) && !terminal.isTopLevelInFlow()) return
        val name = terminal.callableName.asString()
        if (name !in stdlibTerminators) return
        if (expression.argumentList.arguments.isNotEmpty()) return

        val filterCall = filterCallOf(expression.explicitReceiver) ?: return
        val filter = resolvedFunction(filterCall) ?: return
        if (filter.callableName.asString() != "filter") return
        val terminalOk = when {
            filter.isTopLevelIn(stdlibPackages) -> terminal.packageName in stdlibPackages
            filter.isTopLevelInFlow() -> terminal.packageName == flowPackage && name in flowTerminators
            else -> false
        }
        if (!terminalOk) return

        val lambda = trailingLambdaOf(filterCall) ?: return
        val predicate = lambdaText(filterCall, lambda) ?: return
        // A return to the implicit filter label loses its target after the
        // suggested replacement changes the call name.
        if ("return@filter" in predicate) return

        report(expression.source, "Replace '.filter $predicate.$name()' with '.$name $predicate'.")
    }

    // The callable id of the function a call resolves to, or null when it does
    // not resolve or is called through an import alias (the name as written
    // differs from the function's own name), like Go, which compares the
    // names as written.
    private fun resolvedFunction(call: FirFunctionCall): CallableId? {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return null
        val id = symbol.callableId
        if (call.calleeReference.name != id.callableName) return null
        return id
    }

    private fun CallableId.isTopLevelIn(packages: Set<FqName>): Boolean = classId == null && packageName in packages

    private fun CallableId.isTopLevelInFlow(): Boolean = classId == null && packageName == flowPackage

    // The filter's only argument when it is a trailing lambda, also when it
    // is converted to a SAM interface (FileCollection.filter(Spec)).
    private fun trailingLambdaOf(filterCall: FirFunctionCall): FirAnonymousFunctionExpression? {
        var argument = filterCall.argumentList.arguments.singleOrNull() ?: return null
        if (argument is FirSamConversionExpression) argument = argument.expression
        val lambda = argument as? FirAnonymousFunctionExpression ?: return null
        return if (lambda.isTrailingLambda) lambda else null
    }

    private fun filterCallOf(receiver: FirExpression?): FirFunctionCall? = when (receiver) {
        is FirFunctionCall -> receiver
        is FirCheckedSafeCallSubject -> filterCallOf(receiver.originalReceiverRef.value)
        is FirSafeCallExpression -> receiver.selector as? FirFunctionCall
        else -> null
    }

    // Keep an explicit label so return@label remains valid in the suggestion.
    private fun lambdaText(filterCall: FirFunctionCall, lambda: FirAnonymousFunctionExpression): String? {
        val text = (lambda.anonymousFunction.source ?: lambda.source)?.text?.toString() ?: return null
        val brace = text.indexOf('{')
        if (brace < 0) return null
        val callText = filterCall.source?.text?.toString().orEmpty()
        val label = Regex("""([A-Za-z_][A-Za-z_0-9]*@)\s*$""")
            .find(callText.substringBefore('{'))?.groupValues?.get(1)
        return if (label == null) text.substring(brace) else label + text.substring(brace)
    }
}
