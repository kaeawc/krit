package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.Name

/**
 * Flags a URI permission grant on a `Context`: a call of `grantUriPermission`
 * (or `grantUriPermissions`, which the Go rule accepts too) whose resolved
 * function is a member of `Context`, of a subtype of it, or an extension on one
 * of those types.
 *
 * Like the Go rule:
 * - every such call is a finding; the URI and the mode flags are not inspected;
 * - a type counts as a `Context` by its simple name, in any package, or by a
 *   supertype with that simple name (Go's `grantURITypeIsContext` accepts the
 *   simple name `Context` too);
 * - the finding sits on the first line of the call expression, including its
 *   receiver.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go reports every unqualified call named `grantUriPermission`,
 *   and a qualified one whose receiver it cannot type (such as `x!!`). A
 *   same-named member of an unrelated class, a top-level function, or an
 *   invoked function-typed property is not a `Context` URI permission grant,
 *   so this checker does not report it.
 * - Recall: resolution types receivers Go's source inference concludes are
 *   not a `Context` (a `ContextWrapper`, a class extending `ContextWrapper`, a
 *   parenthesized cast to `Context`), so those grants are reported.
 */
internal object GrantAllUris : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "GrantAllUris"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(GrantAllUris)
    }

    private const val MESSAGE = "Overly broad URI permission grant. Consider restricting to specific URIs."
    private val NAMES = setOf(Name.identifier("grantUriPermission"), Name.identifier("grantUriPermissions"))
    private val CONTEXT = Name.identifier("Context")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name !in NAMES) return
        if (!isContextMember(callee)) return
        report(expression.source, MESSAGE)
    }

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; looking one of those up by class id throws.
    context(context: CheckerContext)
    private fun isContextMember(callee: FirFunctionSymbol<*>): Boolean {
        val owner = callee.getContainingClassSymbol()
        if (owner != null && isContextClass(owner)) return true
        // An extension receiver spelled through a typealias is expanded first.
        val receiver = callee.resolvedReceiverType
            ?.fullyExpandedType()
            ?.lowerBoundIfFlexible()
            ?.toClassLikeSymbol(context.session)
            ?: return false
        return isContextClass(receiver)
    }

    context(context: CheckerContext)
    private fun isContextClass(symbol: FirClassLikeSymbol<*>): Boolean =
        symbol.classId.shortClassName == CONTEXT ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId.shortClassName == CONTEXT }
}
