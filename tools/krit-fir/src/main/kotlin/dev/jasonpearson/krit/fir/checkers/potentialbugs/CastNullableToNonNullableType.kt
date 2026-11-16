package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.oracle.ErrorInvolvement
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isNullableOperand
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `expr as T` where the compiler types `expr` as nullable and T as non-nullable, so a null operand fails
// the cast with an NPE-like TypeCastException. `as?` reports null instead.
//
// This mirrors the Go CastNullableToNonNullableType rule, which follows detekt: a `null` literal operand and an
// operand that is itself a cast are not reported, a type parameter with a nullable bound is a nullable target, and
// a platform type or a stable smart cast to non-null is not a nullable operand. The compiler's type replaces Go's
// source inference, so a typealias to a nullable type is a nullable target and a shadowing local has its own type.
internal object CastNullableToNonNullableType : FirTypeOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CastNullableToNonNullableType"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val typeOperatorCallCheckers = setOf(CastNullableToNonNullableType)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        if (expression.operation != FirOperation.AS) return
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val operand = expression.argumentList.arguments.firstOrNull() ?: return
        if (isNullLiteral(operand) || isCast(operand)) return
        // A class from a module with no build output is an error type, which the compiler treats as nullable.
        if (ErrorInvolvement.involvesError(operand)) return
        val target = (expression.conversionTypeRef as? FirResolvedTypeRef)?.coneType?.fullyExpandedType() ?: return
        if (target is ConeErrorType || target.canBeNull(context.session)) return
        if (!operand.isNullableOperand()) return
        report(source, "Casting a nullable type to a non-nullable type. Use 'as?' instead.")
    }

    private fun isNullLiteral(operand: FirExpression): Boolean =
        (unwrapSmartCasts(operand) as? FirLiteralExpression)?.kind == ConstantValueKind.Null

    // detekt reads the left side's type through a nested cast, so `(x as Any?) as T` never reports.
    private fun isCast(operand: FirExpression): Boolean =
        (unwrapSmartCasts(operand) as? FirTypeOperatorCall)?.operation.let { it == FirOperation.AS || it == FirOperation.SAFE_AS }
}
