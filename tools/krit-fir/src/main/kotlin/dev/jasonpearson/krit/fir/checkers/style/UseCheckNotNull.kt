package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `check(x != null)` (and `check(null != x)`, with or without a trailing
// lazy-message lambda) that should be `checkNotNull(x)`.
//
// Mirrors the Go UseCheckNotNull rule:
// - the call has exactly one value argument inside the parentheses; a trailing
//   lambda is allowed, a second argument in the parentheses
//   (`check(x != null, { "msg" })`) is not;
// - that argument, parentheses stripped, is a `!=` comparison (not `!==`)
//   with a `null` literal on either side;
// - the compared value is skipped when its type cannot hold null, the way Go
//   skips a name its resolver resolves to a non-null type. A stable smart cast
//   counts, as Go narrows a name after `if (x == null) return`; an unstable
//   one (a `var` property) does not;
// - the finding is reported on the call, so on the line where the call
//   expression (including a `kotlin.` qualifier) starts.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to kotlin.check. Go matches any call
//   whose name is `check` (a member such as `validator.check(...)`, a local
//   `fun check`, a function-typed value named `check`).
// - Precision: an operand whose type is not nullable is skipped even when
//   Go cannot resolve it by name (a call, a property chain, an inferred
//   local, a lambda or loop parameter) or does not narrow it (a contract,
//   `!!`, `as`, `when`, an equality), because there is no nullable value to
//   pass to checkNotNull.
// - Recall: an import alias of kotlin.check and a backticked `check` still
//   call kotlin.check; Go compares the raw callee text and misses them. A
//   parenthesized `(null)` is still a null literal, and a mutable property
//   keeps its nullable type after an early return, where Go narrows it.
internal object UseCheckNotNull : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseCheckNotNull"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UseCheckNotNull)
    }

    private const val MESSAGE = "Use 'checkNotNull(x)' instead of 'check(x != null)'."

    private val kotlinCheck = CallableId(FqName("kotlin"), Name.identifier("check"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != kotlinCheck) return

        val condition = expression.argumentList.arguments
            .filterNot { it is FirAnonymousFunctionExpression && it.isTrailingLambda }
            .singleOrNull()
            ?.let(::unwrapArgument) as? FirEqualityOperatorCall ?: return
        if (condition.operation != FirOperation.NOT_EQ) return

        val operands = condition.argumentList.arguments
        if (operands.size != 2) return
        val (left, right) = operands
        val value = when {
            isNullLiteral(left) -> right
            isNullLiteral(right) -> left
            else -> return
        }

        val type = if (value is FirSmartCastExpression && !value.isStable) {
            value.originalExpression.resolvedType
        } else {
            value.resolvedType
        }
        if (!type.canBeNull(context.session)) return

        report(expression.source, MESSAGE)
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    private fun isNullLiteral(expression: FirExpression): Boolean =
        expression is FirLiteralExpression && expression.kind == ConstantValueKind.Null
}
