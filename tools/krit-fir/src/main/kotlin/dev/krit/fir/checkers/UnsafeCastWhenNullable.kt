package dev.krit.fir.checkers

import dev.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.isMarkedNullable

// Flags `expr as NullableType?` — unsafe cast to a nullable target type.
// Using `as?` instead silences the rule and avoids ClassCastException at runtime.
internal object UnsafeCastWhenNullable : FirTypeOperatorCallChecker(MppCheckerKind.Common) {

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        if (expression.operation != FirOperation.AS) return

        val source = expression.source ?: return
        val targetType = (expression.conversionTypeRef as? FirResolvedTypeRef)?.coneType ?: return
        if (!targetType.isMarkedNullable) return

        reporter.reportOn(source, KritDiagnostics.UNSAFE_CAST_WHEN_NULLABLE)
    }
}
