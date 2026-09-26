package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirEqualityOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `x.find { ... } != null` / `== null` (and the firstOrNull / lastOrNull
// variants, with `null` on either side) that should be `x.any { ... }` /
// `x.none { ... }`.
//
// Mirrors the Go UseAnyOrNoneInsteadOfFind rule:
// - the comparison is `==` or `!=` (not `===` / `!==`) with a `null`
//   literal on one side;
// - the other side is a call of find / firstOrNull / lastOrNull that passes a
//   predicate lambda (the no-predicate `firstOrNull()` is not reported), on a
//   plain or safe-call (`?.`) receiver;
// - the finding is reported on the comparison, and the message names the
//   function and the operator as written: `Use '.any {}' instead of
//   '.find {} != null'.`
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to the Kotlin standard library's
//   find / firstOrNull / lastOrNull (collections, arrays, sequences, char
//   sequences), which all have an `any` / `none` counterpart with the same
//   predicate. Go matches the callee name alone, so it also reports a member
//   or local extension named `find` whose receiver has no `any` / `none`.
// - Recall: Go needs the call written as `receiver.name { ... }` with the
//   lambda trailing. The checker also reports an implicit receiver (inside
//   `with(list)` or a List subclass), a lambda passed in parentheses (or by
//   name), a parenthesized call, a parenthesized `(null)`, a backticked name,
//   and an import alias of the stdlib function.
internal object UseAnyOrNoneInsteadOfFind : FirEqualityOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseAnyOrNoneInsteadOfFind"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val equalityOperatorCallCheckers = setOf(UseAnyOrNoneInsteadOfFind)
    }

    private val findFunctions = setOf("find", "firstOrNull", "lastOrNull")

    private val stdlibPackages = setOf(
        FqName("kotlin.collections"),
        FqName("kotlin.sequences"),
        FqName("kotlin.text"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirEqualityOperatorCall) {
        val source = expression.source ?: return
        // `when (x) { null -> }` desugars to a comparison with a fake source.
        if (source.kind != KtRealSourceElementKind) return
        val (operator, replacement) = when (expression.operation) {
            FirOperation.NOT_EQ -> "!=" to "any"
            FirOperation.EQ -> "==" to "none"
            else -> return
        }

        val operands = expression.argumentList.arguments
        if (operands.size != 2) return
        val (left, right) = operands
        val callSide = when {
            isNullLiteral(right) -> left
            isNullLiteral(left) -> right
            else -> return
        }
        val call = findCall(callSide) ?: return

        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId.className != null || callableId.packageName !in stdlibPackages) return
        val name = callableId.callableName.asString()
        if (name !in findFunctions) return
        if (!hasPredicateLambda(call)) return

        report(source, "Use '.$replacement {}' instead of '.$name {} $operator null'.")
    }

    private fun findCall(expression: FirExpression): FirFunctionCall? = when (expression) {
        is FirFunctionCall -> expression
        is FirSafeCallExpression -> expression.selector as? FirFunctionCall
        else -> null
    }

    private fun hasPredicateLambda(call: FirFunctionCall): Boolean =
        call.argumentList.arguments.any { argument ->
            val unwrapped = if (argument is FirWrappedArgumentExpression) argument.expression else argument
            unwrapped is FirAnonymousFunctionExpression && unwrapped.anonymousFunction.isLambda
        }

    private fun isNullLiteral(expression: FirExpression): Boolean =
        expression is FirLiteralExpression && expression.kind == ConstantValueKind.Null
}
