package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.resolvedType

/**
 * FIR `FirExpressionChecker<FirSmartCastExpression>` that records the
 * SMART-CAST-refined type (and nullability) of a stable reference at its
 * source position.
 *
 * Without this checker the oracle only saw the *declared* type of a
 * variable/property reference: [OracleQualifiedAccessChecker] visits the
 * inner [`FirPropertyAccessExpression`], whose `resolvedType` is the
 * declaration type (`Any?`), not the data-flow-refined type. FIR models a
 * smart cast as a [`FirSmartCastExpression`] that *wraps* that access, so
 * after `if (o == null) return` the use site is a smart-cast expression of
 * type `Any` (non-null). Go-side rules (CastNullableToNonNullableType, the
 * redundant-null-safety family) ask the oracle "is this reference nullable
 * here?" — they need the smart-cast answer, not the declaration answer.
 *
 * Dedup: the wrapping [`FirSmartCastExpression`] and its inner access map
 * to the same `"line:col"` key (the wrapper is source-transparent). The
 * entry is recorded with [`OracleCollector.addSmartCastExpression`], which
 * replaces a declared-type entry already at that key and is never displaced
 * by a later one, so the smart-cast type wins regardless of FIR traversal
 * order. Call sites stay authoritative because a smart cast wraps a stable
 * reference, not a call.
 *
 * Self-gates on `OracleCollectorRegistry.current() != null` like the other
 * oracle checkers.
 */
internal object OracleSmartCastChecker :
    FirExpressionChecker<FirSmartCastExpression>(MppCheckerKind.Common) {

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirSmartCastExpression) {
        val collector = OracleCollectorRegistry.current() ?: return
        val source = expression.source ?: return
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
        val offsets = collector.offsetsFor(filePath) ?: return

        val startOffset = source.startOffset
        val (line, col) = offsets.lineColAt(startOffset)
        val key = "$line:$col"

        val resolvedType = runCatching { expression.resolvedType }.getOrNull() ?: return
        val typeFqn = resolvedType.renderFqn()
        if (typeFqn.isBlank()) return

        collector.addSmartCastExpression(
            filePath,
            key,
            ExpressionPayload(
                type = typeFqn,
                nullable = resolvedType.isMarkedNullable,
                startByte = offsets.byteOffsetAt(startOffset),
                endByte = offsets.byteOffsetAt(source.endOffset),
                callTarget = null,
                callTargetResolved = false,
                callTargetSuspend = false,
                annotations = emptyList(),
            ),
        )
    }
}
