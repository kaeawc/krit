package dev.jasonpearson.krit.fir.checkers

import dev.jasonpearson.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType

// Flags `expr as NullableType?` where the cast can actually fail — an unsafe
// downcast to a nullable target type. Using `as?` instead avoids a
// ClassCastException at runtime.
//
// A cast is only flagged when the operand's static type is NOT already a
// subtype of the nullable target, i.e. the cast can throw. `null as String?`
// (operand type Nothing?) and `nonNullString as String?` always succeed —
// those are redundant (USELESS_CAST), not unsafe — so they are not flagged.
internal object UnsafeCastWhenNullable : FirTypeOperatorCallChecker(MppCheckerKind.Common) {

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
        if (operandType != null && operandType.isSubtypeOf(targetType, context.session)) return

        reporter.reportOn(source, KritDiagnostics.UNSAFE_CAST_WHEN_NULLABLE)
    }
}
