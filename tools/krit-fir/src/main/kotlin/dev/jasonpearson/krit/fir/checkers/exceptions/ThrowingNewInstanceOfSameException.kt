package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType

// Flags `throw X(e)` inside `catch (e: X)`: the catch builds a new exception of the type it caught from
// the caught one alone, where a plain rethrow or a different type would be meant.
//
// Mirrors the Go ThrowingNewInstanceOfSameException rule, which compares the last written name of the
// thrown constructor with the written catch type and finds the caught variable by identifier text.
// This checker compares resolved classes (type aliases expanded) and follows the catch parameter's
// symbol, so a different class that shares the simple name, a lambda parameter or local that reuses the
// caught name, and a type alias of the caught class are all decided correctly.
//
// Same as Go: the throw is any `throw` inside the catch body (lambdas and nested catches included); the
// constructor takes exactly one argument; that argument is the caught variable itself, or a member chain
// on it (`e.cause`, `e?.message`), optionally passed by name; a `_` parameter is never reported. The
// finding sits on the `throw`.
//
// Deliberate differences from Go: the thrown value must be a constructor call (Go also reports a
// same-named factory function), and an argument that only starts with the caught name textually
// (`e.message + "!"`, `e.message ?: ""`) adds context, so it is not a wrapper of the caught exception.
internal object ThrowingNewInstanceOfSameException :
    FirExpressionChecker<FirThrowExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "ThrowingNewInstanceOfSameException"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val throwExpressionCheckers = setOf(ThrowingNewInstanceOfSameException)
    }

    private const val MESSAGE =
        "Avoid catch blocks that rethrow a caught exception wrapped inside a new instance of the same exception."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirThrowExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val call = unwrapSmartCasts(expression.exception) as? FirFunctionCall ?: return
        if (call.calleeReference.toResolvedCallableSymbol() !is FirConstructorSymbol) return
        val argument = singleArgument(call) ?: return
        val caught = caughtParameterOf(argument) ?: return
        val caughtClass = caught.parameter.returnTypeRef.coneType.expandedClassType()?.lookupTag?.classId ?: return
        val thrownClass = call.resolvedType.expandedClassType()?.lookupTag?.classId ?: return
        if (caughtClass != thrownClass) return
        report(source, MESSAGE)
    }

    private fun singleArgument(call: FirFunctionCall): FirExpression? {
        var arguments = call.argumentList.arguments
        (arguments.singleOrNull() as? FirVarargArgumentsExpression)?.let { arguments = it.arguments }
        return arguments.singleOrNull()?.let(::unwrapArgument)
    }

    // The catch whose parameter roots [argument], when the argument is that parameter or a member chain on it.
    context(context: CheckerContext)
    private fun caughtParameterOf(argument: FirExpression): FirCatch? {
        val root = chainRoot(argument) ?: return null
        val symbol = root.calleeReference.toResolvedVariableSymbol() ?: return null
        return context.containingElements.lastOrNull { it is FirCatch && it.parameter.symbol == symbol } as? FirCatch
    }

    // The receiver-less property access at the start of a chain of member accesses, calls and safe calls.
    private fun chainRoot(expression: FirExpression): FirPropertyAccessExpression? {
        var current = unwrapSmartCasts(expression)
        while (true) {
            current = when (current) {
                is FirSafeCallExpression -> unwrapSmartCasts(current.receiver)
                is FirCheckNotNullCall ->
                    unwrapSmartCasts(current, throughNotNull = true).takeIf { it != current } ?: return null
                is FirPropertyAccessExpression -> {
                    val receiver = current.explicitReceiver ?: return current
                    unwrapSmartCasts(receiver)
                }
                // `e.message + "!"` is a call to plus with a receiver, but it builds a new value.
                is FirQualifiedAccessExpression ->
                    if (current.source?.elementType == KtNodeTypes.BINARY_EXPRESSION) return null
                    else unwrapSmartCasts(current.explicitReceiver ?: return null)
                else -> return null
            }
        }
    }
}
