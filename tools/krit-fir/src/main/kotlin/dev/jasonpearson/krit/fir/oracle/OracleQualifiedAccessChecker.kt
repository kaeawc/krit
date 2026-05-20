package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.resolvedType

/**
 * FIR `FirExpressionChecker<FirQualifiedAccessExpression>` that
 * captures resolved-type info for every property / variable access
 * the K2 frontend visits. Powers `Resolver.expressionType(expr)`
 * queries from plugin rules running on the krit-fir backend for
 * non-call expressions — without this checker, only
 * [`FirFunctionCall`] sites had a populated [`ExpressionPayload.type`]
 * and rules asking about `someVar`, `obj.field`, etc. saw null.
 *
 * The checker writes the same payload shape `OracleExpressionChecker`
 * uses (just with `callTarget = null` and `callTargetResolved = false`),
 * so the [`OracleCollector.addExpression`] first-wins dedup keeps
 * call-site entries authoritative when a property access shares the
 * same `"line:col"` (e.g. the receiver of a chained call). The
 * resolver's call-specific methods still ignore non-call entries
 * via the `callTargetResolved` gate.
 *
 * Self-gates on `OracleCollectorRegistry.current() != null` like the
 * other oracle checkers — non-oracle compilation paths pay only the
 * thread-local probe.
 */
internal object OracleQualifiedAccessChecker :
    FirExpressionChecker<FirQualifiedAccessExpression>(MppCheckerKind.Common) {

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirQualifiedAccessExpression) {
        // FirFunctionCall is a subtype of FirQualifiedAccessExpression
        // in FIR. OracleExpressionChecker already handles the call
        // case with full callTarget / suspend / annotations metadata,
        // so let it own that path; we only need to fill in the gap
        // for non-call qualified accesses.
        if (expression is FirFunctionCall) return

        val collector = OracleCollectorRegistry.current() ?: return
        val source = expression.source ?: return
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
        val offsets = collector.offsetsFor(filePath) ?: return

        val startOffset = source.startOffset
        val (line, col) = offsets.lineColAt(startOffset)
        val key = "$line:$col"
        if (collector.hasExpression(filePath, key)) return

        val resolvedType = runCatching { expression.resolvedType }.getOrNull() ?: return
        val typeFqn = resolvedType.renderFqn()
        if (typeFqn.isBlank()) return

        collector.addExpression(
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
