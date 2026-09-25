package dev.jasonpearson.krit.fir.checkers.compose

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.FqName

// Flags `remember { f(param) }`: a keyless remember whose calculation captures a
// parameter of the enclosing function. Because the memo has no keys, the cached
// value never recomputes when that parameter changes across recomposition, so
// the composable renders stale data.
//
// A keyless remember that captures nothing external (`remember { mutableStateOf(0) }`,
// `remember { Paint() }`) is the correct, idiomatic form and must not be
// flagged. This mirrors the Go ComposeRememberWithoutKey rule, which fires
// only when the calculation references an enclosing function parameter.
internal object ComposeRememberWithoutKey : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposeRememberWithoutKey"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ComposeRememberWithoutKey)
    }
    private val rememberFqName = FqName("androidx.compose.runtime.remember")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return

        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId?.asSingleFqName() != rememberFqName) return

        // remember(vararg keys, calculation) — a single argument is the
        // keyless overload; any keys mean recomposition is already handled.
        val argument = expression.argumentList.arguments.singleOrNull() ?: return

        val enclosingParams = nearestEnclosingFunctionParameters() ?: return
        if (enclosingParams.isEmpty()) return

        // Walk the whole calculation argument, not just a lambda body: this
        // also covers a bound callable reference (`remember(model::toString)`),
        // whose receiver captures the enclosing parameter just as a lambda
        // capture would.
        if (!capturesEnclosingParam(unwrapArgument(argument), enclosingParams)) return

        report(source, "remember { ${callee.name.asString()} } is missing an explicit key argument.")
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    // Value-parameter symbols of the function that lexically contains the
    // remember call — the nearest enclosing NAMED function. Anonymous
    // functions (Compose content lambdas such as `Column { ... }`) also appear
    // in containingDeclarations, so filtering FirFunctionSymbol would pick the
    // wrapping lambda (no value parameters) and silently miss the capture.
    // FirNamedFunctionSymbol mirrors the Go rule's nearest
    // "function_declaration" anchor. Returns null when there is no enclosing
    // named function.
    context(context: CheckerContext)
    private fun nearestEnclosingFunctionParameters(): Set<FirBasedSymbol<*>>? {
        val enclosing = context.containingDeclarations
            .filterIsInstance<FirNamedFunctionSymbol>()
            .lastOrNull() ?: return null
        return enclosing.valueParameterSymbols.toSet()
    }

    // True when any resolved reference inside the calculation argument resolves
    // to one of the captured symbols. For a callable reference the referenced
    // callee is not a capture, but its receiver expression is walked as a child
    // and matches. Symbol identity handles shadowing: a nested lambda parameter
    // with the same name is a different symbol and does not match.
    private fun capturesEnclosingParam(argument: FirElement, captured: Set<FirBasedSymbol<*>>): Boolean {
        var found = false
        argument.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirQualifiedAccessExpression) {
                    val symbol = element.calleeReference.toResolvedCallableSymbol()
                    if (symbol != null && symbol in captured) {
                        found = true
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
