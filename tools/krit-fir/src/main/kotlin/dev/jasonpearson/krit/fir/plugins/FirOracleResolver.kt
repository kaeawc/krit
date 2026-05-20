package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.Resolver
import dev.jasonpearson.krit.fir.oracle.ExpressionPayload
import dev.jasonpearson.krit.fir.oracle.FileOffsetTable
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtExpression
import org.jetbrains.kotlin.psi.KtLambdaExpression

/**
 * FIR-backed [`Resolver`] for plugin-rule hosting on the krit-fir
 * backend. Answers PSI-keyed questions about a single analyzed file by
 * looking up data the K2 oracle pass already collected (see
 * `OracleExpressionChecker`).
 *
 * Resolution flow per query:
 *
 * 1. The PSI element's `textRange.startOffset` is mapped to 1-based
 *    `(line, col)` via the file's [FileOffsetTable].
 * 2. The resulting `"line:col"` key is looked up in [expressionsByKey],
 *    which is the slice of an [`AnalyzeResult.files`] payload for the
 *    file under analysis.
 *
 * The [Resolver] surface still exposes `isLambdaSuspend` and
 * `expressionType`; the FIR oracle pass doesn't capture lambda type
 * or expression type today, so those methods return the conservative
 * "unknown" value (`false` / `null`). Rules that strictly depend on
 * either can opt out via the `NEEDS_FIR` capability — when those
 * facts land the resolver picks them up here without an SPI churn.
 */
internal class FirOracleResolver(
    private val expressionsByKey: Map<String, ExpressionPayload>,
    private val offsets: FileOffsetTable,
) : Resolver {

    override fun isSuspendCall(call: KtCallExpression): Boolean =
        lookupCall(call)?.callTargetSuspend == true

    override fun resolvedCallFqName(call: KtCallExpression): String? {
        val payload = lookupCall(call) ?: return null
        // krit-types' AnalysisApiResolver returns null when the symbol
        // graph couldn't recover an FQN. Mirror that here by also
        // gating on `callTargetResolved` — an entry that fell back to
        // the lexical callee text isn't a real FQN.
        if (!payload.callTargetResolved) return null
        return payload.callTarget?.takeIf { it.isNotBlank() }
    }

    override fun isLambdaSuspend(lambda: KtLambdaExpression): Boolean {
        // The oracle pass doesn't capture lambda-functional-type
        // information today. Returning false matches krit-types'
        // "unresolved → conservative false" contract.
        return false
    }

    override fun expressionType(expression: KtExpression): String? {
        // Same gap as isLambdaSuspend: expression-type data isn't part
        // of the current ExpressionPayload surface. krit-types'
        // resolver also returns null for unresolved types.
        return null
    }

    private fun lookupCall(call: KtCallExpression): ExpressionPayload? {
        val range = call.textRange ?: return null
        val (line, col) = offsets.lineColAt(range.startOffset)
        return expressionsByKey["$line:$col"]
    }
}
