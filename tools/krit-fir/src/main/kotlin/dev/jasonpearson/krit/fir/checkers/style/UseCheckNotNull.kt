package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isNullLiteral
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.trailingLambdaOf
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `check(x != null)` (and `check(null != x)`, with or without a trailing
// lazy-message lambda) that should be `checkNotNull(x)`.
//
// Mirrors the Go UseCheckNotNull rule:
// - the first value argument, parentheses stripped, is a `!=` comparison
//   (not `!==`) with a `null` literal on either side; a lazy message may
//   follow inside the parentheses (`check(x != null, { "msg" })`) or as a
//   trailing lambda, as detekt reads only the first argument;
// - the compared value is skipped when its type cannot hold null, the way Go
//   skips a name its resolver resolves to a non-null type. A stable smart cast
//   counts, as Go narrows a name after `if (x == null) return`; an unstable
//   one (a `var` property) does not;
// - the finding is reported on the call expression after any `kotlin.`
//   qualifier, so it starts at the callee name, as in Go and detekt.
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

    private const val MESSAGE = "Use checkNotNull() instead of check() for checking not-null."

    private val kotlinCheck = CallableId(FqName("kotlin"), Name.identifier("check"))
    private val VALUE = Name.identifier("value")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != kotlinCheck) return

        val arguments = expression.argumentList.arguments
            .filterNot { trailingLambdaOf(it) != null }
        if (arguments.size > 2) return
        val valueArgument = expression.resolvedArgumentMapping?.entries?.firstOrNull { it.value.name == VALUE }?.key
            ?: arguments.firstOrNull() ?: return
        // The resolved list follows the parameters, so compare source offsets for the first written argument.
        val start = valueArgument.source?.startOffset ?: return
        if (arguments.any { it !== valueArgument && (it.source?.startOffset ?: Int.MAX_VALUE) < start }) return
        val condition = unwrapArgument(valueArgument) as? FirEqualityOperatorCall ?: return
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

        val source = expression.source ?: return
        report(selectorCallExpression(source)?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
    }
}
