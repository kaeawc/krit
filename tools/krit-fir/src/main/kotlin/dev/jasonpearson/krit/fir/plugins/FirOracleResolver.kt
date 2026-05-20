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
 * looking up data the K2 oracle pass already collected.
 *
 * Resolution flow per query:
 *
 * 1. The PSI element's `textRange.startOffset` is mapped to 1-based
 *    `(line, col)` via the file's [FileOffsetTable].
 * 2. The resulting `"line:col"` key is looked up in the corresponding
 *    pre-computed map ([expressionsByKey] for call resolution and
 *    expression types, [lambdaSuspendByKey] for lambda functional
 *    suspend status).
 *
 * `expressionType` for non-call expressions still returns null — the
 * oracle pass only captures resolved types for [FirFunctionCall] node
 * positions today. Most rules query call-expression types; broadening
 * to all FirExpression subtypes is a follow-up.
 */
internal class FirOracleResolver(
    private val expressionsByKey: Map<String, ExpressionPayload>,
    private val offsets: FileOffsetTable,
    private val lambdaSuspendByKey: Map<String, Boolean> = emptyMap(),
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
        // The PSI offset for a lambda expression sits on the opening
        // brace. K2 records the FirAnonymousFunction's source against
        // the same offset, so a `"line:col"` round-trip lookup matches
        // the visited declaration.
        val range = lambda.textRange ?: return false
        val (line, col) = offsets.lineColAt(range.startOffset)
        return lambdaSuspendByKey["$line:$col"] == true
    }

    override fun expressionType(expression: KtExpression): String? {
        // The oracle pass populates [`ExpressionPayload.type`] for
        // function calls (via OracleExpressionChecker) and qualified
        // accesses — property reads, variable refs, receivers in
        // call chains (via OracleQualifiedAccessChecker). Both write
        // entries into `expressionsByKey` keyed by `"line:col"`, so a
        // single PSI offset → line:col lookup serves either source.
        // Expressions outside that set (literals, when expressions,
        // tries, etc.) still return null — broadening coverage is
        // purely additive on the writer side and doesn't change the
        // resolver contract.
        val range = expression.textRange ?: return null
        val (line, col) = offsets.lineColAt(range.startOffset)
        return expressionsByKey["$line:$col"]?.type?.takeIf { it.isNotBlank() }
    }

    private fun lookupCall(call: KtCallExpression): ExpressionPayload? {
        val range = call.textRange ?: return null
        val (line, col) = offsets.lineColAt(range.startOffset)
        return expressionsByKey["$line:$col"]
    }
}
