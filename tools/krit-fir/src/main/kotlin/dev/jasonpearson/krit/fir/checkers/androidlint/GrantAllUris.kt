package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.Name

/**
 * Flags a URI permission grant on a `Context`: a call of `grantUriPermission`
 * (or `grantUriPermissions`, which the Go rule accepts too) that is made on a
 * `Context`. That is a call whose resolved function is a member of `Context` (or
 * of a subtype), or an extension on one of those types, or whose actual
 * receiver is a `Context`.
 *
 * Like the Go rule:
 * - every such call is a finding; the URI and the mode flags are not inspected;
 * - a type counts as a `Context` by its simple name, in any package, or by a
 *   supertype with that simple name (Go's `grantURITypeIsContext` accepts the
 *   simple name `Context` too); a type parameter counts when one of its bounds
 *   does;
 * - the receiver decides, not where the function is declared: a same-named
 *   function declared elsewhere (an interface a `Context` subclass implements,
 *   an extension on `Any`) called on a `Context` is reported, and a member
 *   extension on a `String` declared inside an `Activity` is not;
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
 *   parenthesized cast to `Context`, a type parameter bounded by `Context`), so
 *   those grants are reported.
 */
internal object GrantAllUris : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "GrantAllUris"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(GrantAllUris)
    }

    private const val MESSAGE = "Overly broad URI permission grant. Consider restricting to specific URIs."
    private val NAMES = setOf(Name.identifier("grantUriPermission"), Name.identifier("grantUriPermissions"))
    private val CONTEXT = Name.identifier("Context")
    private const val MAX_TYPE_DEPTH = 16

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name !in NAMES) return
        if (!isContextCall(expression, callee)) return
        report(expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isContextCall(expression: FirFunctionCall, callee: FirFunctionSymbol<*>): Boolean {
        // An extension decides on its extension receiver alone: a member
        // extension on a String inside an Activity is called on the String.
        val declaredReceiver = callee.resolvedReceiverType
        if (declaredReceiver != null) {
            return isContextType(declaredReceiver, 0) || isContextReceiver(expression.extensionReceiver)
        }
        // The containing class comes from the symbol's lookup tag, which is
        // bound to local and anonymous classes; looking one of those up by
        // class id throws.
        val owner = callee.getContainingClassSymbol()
        if (owner != null && isContextClass(owner)) return true
        return isContextReceiver(expression.dispatchReceiver)
    }

    context(context: CheckerContext)
    private fun isContextReceiver(receiver: FirExpression?): Boolean {
        val type = receiver?.let { runCatching { it.resolvedType }.getOrNull() } ?: return false
        return isContextType(type, 0)
    }

    // A typealias is expanded first; a type parameter is a Context when one of
    // its bounds is.
    context(context: CheckerContext)
    private fun isContextType(type: ConeKotlinType, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        return when (val expanded = type.fullyExpandedType().lowerBoundIfFlexible()) {
            is ConeDefinitelyNotNullType -> isContextType(expanded.original, depth + 1)
            is ConeIntersectionType -> expanded.intersectedTypes.any { isContextType(it, depth + 1) }
            is ConeTypeParameterType -> expanded.lookupTag.typeParameterSymbol.resolvedBounds.any {
                isContextType(it.coneType, depth + 1)
            }
            is ConeClassLikeType -> expanded.toClassLikeSymbol(context.session)?.let { isContextClass(it) } == true
            else -> false
        }
    }

    context(context: CheckerContext)
    private fun isContextClass(symbol: FirClassLikeSymbol<*>): Boolean =
        symbol.classId.shortClassName == CONTEXT ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId.shortClassName == CONTEXT }
}
