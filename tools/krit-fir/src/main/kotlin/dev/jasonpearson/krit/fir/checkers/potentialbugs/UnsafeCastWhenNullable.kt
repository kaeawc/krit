package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.nullableClassType
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeStarProjection
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.constructClassType
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.StandardClassIds

// Flags `expr as NullableType?` where the cast can actually fail — an unsafe
// downcast to a nullable target type. Using `as?` instead avoids a
// ClassCastException at runtime.
//
// A cast is only flagged when the operand's static type is NOT already a
// subtype of the nullable target, i.e. the cast can throw. `null as String?`
// (operand type Nothing?) and `nonNullString as String?` always succeed —
// those are redundant (USELESS_CAST), not unsafe — so they are not flagged.
// The JVM checks only the erased target, so an unchecked cast (`x as T?` for a
// non-reified T, `list as List<String>?`) is also skipped when the operand
// already fits the erasure: it cannot throw, and `as?` would not return null.
internal object UnsafeCastWhenNullable : FirTypeOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnsafeCastWhenNullable"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val typeOperatorCallCheckers = setOf(UnsafeCastWhenNullable)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        if (expression.operation != FirOperation.AS) return

        val source = expression.source ?: return
        val targetType = (expression.conversionTypeRef as? FirResolvedTypeRef)?.coneType ?: return
        if (!targetType.isMarkedNullable) return

        // Skip casts that always succeed: when the operand's type is already a
        // subtype of the nullable target, the cast can never throw, so it is
        // redundant rather than unsafe. Only reachable-failure casts are flagged.
        val operandType = runCatching { expression.argumentList.arguments.firstOrNull()?.resolvedType }.getOrNull()
        if (operandType != null) {
            if (operandType.isSubtypeOf(targetType, context.session)) return
            val erased = erasedNullableTarget(targetType)
            if (erased != null && operandType.isSubtypeOf(erased, context.session)) return
        }

        report(source, "Unsafe cast to nullable type; prefer 'as?' to avoid ClassCastException at runtime.")
    }

    // The nullable type the JVM checks at runtime: a non-reified type parameter
    // becomes its first bound, and class type arguments become star projections.
    // Arrays keep their element type because the JVM checks it. Null when the
    // erasure is not modelled here, so the caller keeps the finding.
    context(context: CheckerContext)
    private fun erasedNullableTarget(target: ConeKotlinType): ConeKotlinType? {
        var type = target.lowerBoundIfFlexible().fullyExpandedType()
        var depth = 0
        while (type is ConeTypeParameterType) {
            val parameter = type.lookupTag.typeParameterSymbol
            if (parameter.isReified || depth++ == MAX_BOUND_DEPTH) return null
            val bound = parameter.resolvedBounds.firstOrNull()?.coneType
                ?: return StandardClassIds.Any.nullableClassType()
            type = bound.lowerBoundIfFlexible().fullyExpandedType()
        }
        val classType = type as? ConeClassLikeType ?: return null
        if (classType is ConeErrorType || classType.lookupTag.classId == StandardClassIds.Array) return null
        return classType.lookupTag.constructClassType(
            Array(classType.typeArguments.size) { ConeStarProjection },
            isMarkedNullable = true,
        )
    }
}

// Bounds deeper than this (`T : U, U : V, ...`) are left unerased.
private const val MAX_BOUND_DEPTH = 8
