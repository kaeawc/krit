package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals

/**
 * FIR `FirFunctionCallChecker` that projects each visited
 * [FirFunctionCall] into an [ExpressionPayload] and pushes it onto the
 * active [OracleCollector], if any. Mirrors krit-types' per-call
 * projection: only callable resolution is exposed (no expression type
 * or nullability), since the Go-side rules consuming this data only
 * read `callTarget`, `callTargetSuspend`, and annotations.
 *
 * The checker gates itself on `OracleCollectorRegistry.current() != null`
 * so plugin-only compilations (the diagnostic `check()` command) pay
 * only a thread-local probe per call.
 */
internal object OracleExpressionChecker : FirFunctionCallChecker(MppCheckerKind.Common) {

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val collector = OracleCollectorRegistry.current() ?: return
        val source = expression.source ?: return
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
        val offsets = collector.offsetsFor(filePath) ?: return

        val startOffset = source.startOffset
        val endOffset = source.endOffset
        val (line, col) = offsets.lineColAt(startOffset)
        val key = "$line:$col"
        if (collector.hasExpression(filePath, key)) return

        val callee = expression.calleeReference.toResolvedCallableSymbol()
        val lexicalFallback = expression.calleeReference.name.asString()

        var callTarget: String = lexicalFallback
        var callTargetResolved = false
        var callTargetSuspend = false
        var annotations: List<String> = emptyList()
        if (callee != null) {
            val fqn = callee.callableId?.asSingleFqName()?.asString().orEmpty()
            if (fqn.isNotEmpty()) {
                callTarget = fqn
                callTargetResolved = true
                callTargetSuspend = callee.isSuspend
                annotations = callee.annotations.mapNotNull {
                    it.toAnnotationClassId(context.session)?.asSingleFqName()?.asString()
                }
            }
        }

        if (callTarget.isEmpty()) return

        val payload = ExpressionPayload(
            // type / nullable stay empty to match krit-types' shape; the
            // downstream Go-side oracle consumers only read callTarget +
            // annotations.
            type = "",
            nullable = false,
            startByte = offsets.byteOffsetAt(startOffset),
            endByte = offsets.byteOffsetAt(endOffset),
            callTarget = callTarget,
            callTargetResolved = callTargetResolved,
            callTargetSuspend = callTargetSuspend,
            annotations = annotations,
        )
        collector.addExpression(filePath, key, payload)
    }
}
