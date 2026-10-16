@file:OptIn(KaExperimentalApi::class, KaContextParameterApi::class, KtExperimentalApi::class)

package dev.jasonpearson.krit.types

import org.jetbrains.kotlin.analysis.api.KaContextParameterApi
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.resolution.KaCallResolutionError
import org.jetbrains.kotlin.analysis.api.resolution.KaCallResolutionSuccess
import org.jetbrains.kotlin.analysis.api.resolution.KaFunctionCall
import org.jetbrains.kotlin.analysis.api.resolution.KaMultiCallResolutionAttempt
import org.jetbrains.kotlin.analysis.api.resolution.KaSingleCall
import org.jetbrains.kotlin.analysis.api.resolution.KaVariableAccessCall
import org.jetbrains.kotlin.analysis.api.resolution.calls
import org.jetbrains.kotlin.analysis.api.resolution.symbol
import org.jetbrains.kotlin.analysis.api.resolution.symbols
import org.jetbrains.kotlin.analysis.api.resolution.tryResolveSymbols
import org.jetbrains.kotlin.analysis.api.resolution.tryResolveCall
import org.jetbrains.kotlin.analysis.api.symbols.KaCallableSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbol
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtElement
import org.jetbrains.kotlin.psi.KtExperimentalApi
import org.jetbrains.kotlin.psi.KtForExpression
import org.jetbrains.kotlin.psi.KtPropertyDelegate
import org.jetbrains.kotlin.resolution.KtResolvable

// Internal adapter over the Analysis API's element-based resolution entry
// points (`tryResolveCall`, `KaCallResolutionSuccess` / `KaCallResolutionError`).
//
// Every call-site consumer in krit-types goes through here so that the
// distinction the Analysis API draws between a *successful* call and a *sole
// candidate of a failed call* is stated at the use site instead of being
// implied by which reduction helper happened to be called. Analysis API
// objects (symbols, calls, attempts) are lifetime-bound to a KaSession and
// never leave this package; the public plugin `Resolver` API only exposes
// strings and booleans. The element-based resolution API is still
// @KaExperimentalApi in the pinned compiler (2.4.20), so the opt-in is confined
// to this file and a future API move is a one-file change.

/**
 * How a consumer treats a call that did not resolve cleanly.
 *
 * - [SUCCESS_ONLY]: only a successfully resolved call counts. A failed call
 *   (inapplicable argument, ambiguity, unresolved reference) yields no target.
 * - [CANDIDATE_TOLERANT]: a failed call with exactly one candidate yields that
 *   candidate, exactly as the `singleFunctionCallOrNull()` /
 *   `singleVariableAccessCall()` reductions of the former `KaCallInfo` did.
 *   Several candidates (ambiguity) or none still yield no target.
 *
 * Current consumers, all [CANDIDATE_TOLERANT] because they are best-effort
 * lookups over possibly incomplete editor code, and their result is advisory
 * (a call-target hint for rules; a lexical fallback exists when absent):
 *
 *  - Main.kt `analyzeKtFile` expression export (callTarget FQN, suspend
 *    marker, annotations);
 *  - PluginRules.kt `AnalysisApiResolver.resolveCallTarget` (`isSuspendCall`,
 *    `resolvedCallFqName`);
 *  - PluginRules.kt `callHasSuspendFunctionalParam` (`isLambdaSuspend`).
 *
 * A consumer that needs an authoritative answer must use [SUCCESS_ONLY];
 * [CallTarget.certainty] lets tolerant consumers tell the two apart.
 */
internal enum class CallResolutionPolicy { SUCCESS_ONLY, CANDIDATE_TOLERANT }

/** Whether a [CallTarget] came from a successful call or a sole failed-call candidate. */
internal enum class CallCertainty { SUCCESS, CANDIDATE }

/** The callable a call expression targets. Only valid inside the producing session. */
internal class CallTarget(val symbol: KaCallableSymbol, val certainty: CallCertainty)

/**
 * Resolves [call] to a function or variable-access target under [policy], or
 * returns null when there is no such unique target (unresolved, ambiguous,
 * not a callable member call, or a failure under [CallResolutionPolicy.SUCCESS_ONLY]).
 */
internal fun KaSession.resolveCallTarget(call: KtCallExpression, policy: CallResolutionPolicy): CallTarget? {
    val (single, certainty) = when (val attempt = call.tryResolveCall()) {
        is KaCallResolutionSuccess -> attempt.call to CallCertainty.SUCCESS
        is KaCallResolutionError -> {
            if (policy != CallResolutionPolicy.CANDIDATE_TOLERANT) return null
            (attempt.candidateCalls.singleOrNull() ?: return null) to CallCertainty.CANDIDATE
        }
        // Multi-call attempts (compound operations) never target a single callable.
        else -> return null
    }
    if (single !is KaFunctionCall<*> && single !is KaVariableAccessCall) return null
    return CallTarget(single.callableSymbol(), certainty)
}

private fun KaSingleCall<*, *>.callableSymbol(): KaCallableSymbol = symbol

/**
 * Every symbol [element] refers to, for dependency tracking: the symbols of a
 * name, operator or array access, and for the elements whose operator
 * conventions have no name at the use site (`for`, property delegates) the
 * functions those conventions resolve to.
 *
 * Candidate-tolerant by contract: when resolution fails, the candidate symbols
 * are still returned (`KaSymbolResolutionAttempt.symbols`, as opposed to
 * `successfulSymbols`). A dependency edge to a declaration that an edit could
 * turn into a successful resolution is exactly what the cache must keep, so
 * this must never be narrowed to successful symbols.
 */
internal fun KaSession.referencedSymbols(element: KtElement): List<KaSymbol> = when (element) {
    is KtForExpression -> multiCallSymbols(element.tryResolveCall())
    is KtPropertyDelegate -> multiCallSymbols(element.tryResolveCall())
    is KtResolvable -> element.tryResolveSymbols()?.symbols.orEmpty()
    else -> emptyList()
}

private fun multiCallSymbols(attempt: KaMultiCallResolutionAttempt?): List<KaSymbol> =
    attempt?.attempts.orEmpty().flatMap { it.calls.flatMap { call -> call.symbols } }
