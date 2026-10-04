package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirEqualityOperatorCallChecker
import org.jetbrains.kotlin.fir.declarations.FirResolvePhase
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.resolve.scope
import org.jetbrains.kotlin.fir.scopes.CallableCopyTypeCalculator
import org.jetbrains.kotlin.fir.scopes.getFunctions
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeStarProjection
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSomeFunctionType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.typeContext
import org.jetbrains.kotlin.fir.types.withNullability
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `x.find { ... } != null` / `== null` (and the firstOrNull / lastOrNull
// variants, with `null` on either side) that should be `x.any { ... }` /
// `x.none { ... }`.
//
// Mirrors the Go UseAnyOrNoneInsteadOfFind rule:
// - the comparison is `==` or `!=` (not `===` / `!==`) with a `null`
//   literal on one side;
// - the other side is a call of find / firstOrNull / lastOrNull that passes a
//   predicate lambda (the no-predicate `firstOrNull()` is not reported), on a
//   plain or safe-call (`?.`) receiver;
// - the finding is reported on the comparison, and the message names the
//   function and the operator as written: `Use '.any {}' instead of
//   '.find {} != null'.`
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the receiver must have the suggested replacement, an `any`
//   (for `!=`) or `none` (for `==`) that takes a predicate. The checker looks
//   for it as a member of the receiver type (androidx.collection's ObjectList
//   and ScatterSet, a user class declaring its own find and any / none), or as
//   an extension whose receiver the receiver type is a subtype of, declared in
//   the standard library's collections, sequences and text packages or in the
//   package of the find itself (kotlinx.coroutines.flow's Flow.any / none,
//   present from coroutines 1.10). Go matches the callee name alone, so it
//   also reports a find on a receiver with no `any` / `none` to use instead,
//   and a `== null` on ObjectList / ScatterSet, which have only a
//   predicate-less `none()`.
// - Recall: Go needs the call written as `receiver.name { ... }` with the
//   lambda trailing. The checker also reports an implicit receiver (inside
//   `with(list)` or a List subclass), a lambda passed in parentheses (or by
//   name), a parenthesized call, a parenthesized `(null)`, a backticked name,
//   and an import alias of the stdlib function.
internal object UseAnyOrNoneInsteadOfFind : FirEqualityOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseAnyOrNoneInsteadOfFind"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val equalityOperatorCallCheckers = setOf(UseAnyOrNoneInsteadOfFind)
    }

    private val findFunctions = setOf("find", "firstOrNull", "lastOrNull")

    private val anyName = Name.identifier("any")
    private val noneName = Name.identifier("none")

    // Where the standard library declares its any / none extensions.
    private val stdlibPackages = listOf(
        FqName("kotlin.collections"),
        FqName("kotlin.sequences"),
        FqName("kotlin.text"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirEqualityOperatorCall) {
        val source = expression.source ?: return
        // `when (x) { null -> }` desugars to a comparison with a fake source.
        if (source.kind != KtRealSourceElementKind) return
        val (operator, replacement) = when (expression.operation) {
            FirOperation.NOT_EQ -> "!=" to anyName
            FirOperation.EQ -> "==" to noneName
            else -> return
        }

        val operands = expression.argumentList.arguments
        if (operands.size != 2) return
        val (left, right) = operands
        val callSide = when {
            isNullLiteral(right) -> left
            isNullLiteral(left) -> right
            else -> return
        }
        val call = findCall(callSide) ?: return

        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        val name = callableId.callableName.asString()
        if (name !in findFunctions) return
        if (!hasPredicateLambda(call)) return
        if (!hasPredicateCounterpart(call, callee, callableId.packageName, replacement)) return

        report(source, "Use '.$replacement {}' instead of '.$name {} $operator null'.")
    }

    // Whether `replacement { ... }` resolves for the receiver the find was
    // called on: a member of the receiver type, or an extension declared in the
    // standard library or next to the find itself whose receiver the receiver
    // type is a subtype of. Only a function whose single parameter is a
    // function type counts: ObjectList.none() takes no predicate.
    context(context: CheckerContext)
    private fun hasPredicateCounterpart(
        call: FirFunctionCall,
        callee: FirCallableSymbol<*>,
        calleePackage: FqName,
        replacement: Name,
    ): Boolean {
        val receiver = if (callee.receiverParameterSymbol != null) call.extensionReceiver else call.dispatchReceiver
        val receiverType = receiver?.resolvedType?.fullyExpandedType() ?: return false

        // Members, inherited ones included. The type scope is the one call
        // resolution uses, bound to local and anonymous classes through their
        // lookup tags; no class id is resolved through the symbol provider.
        val scope = receiverType.lowerBoundIfFlexible().withNullability(false, context.session.typeContext).scope(
            context.session,
            context.scopeSession,
            CallableCopyTypeCalculator.CalculateDeferredForceLazyResolution,
            FirResolvePhase.STATUS,
        )
        if (scope != null &&
            scope.getFunctions(replacement).any { it.receiverParameterSymbol == null && takesPredicate(it) }
        ) {
            return true
        }

        // Top-level extensions: the standard library's, and those in the find's
        // own package (kotlinx.coroutines.flow ships Flow.any / none from 1.10;
        // an older version without them stays silent).
        val provider = context.session.symbolProvider
        return (stdlibPackages + calleePackage).distinct().any { pkg ->
            provider.getTopLevelFunctionSymbols(pkg, replacement).any { candidate ->
                takesPredicate(candidate) && receiverAccepts(candidate, receiverType)
            }
        }
    }

    private fun takesPredicate(function: FirNamedFunctionSymbol): Boolean {
        val parameter = function.valueParameterSymbols.singleOrNull() ?: return false
        return parameter.resolvedReturnType.fullyExpandedType(parameter.moduleData.session)
            .lowerBoundIfFlexible()
            .isSomeFunctionType(parameter.moduleData.session)
    }

    // The extension's receiver, with its type arguments star-projected, is a
    // supertype of the receiver type. The subtype check walks the receiver
    // type's supertypes; it never resolves a class id through the symbol
    // provider.
    context(context: CheckerContext)
    private fun receiverAccepts(extension: FirNamedFunctionSymbol, receiverType: ConeKotlinType): Boolean {
        val declared = extension.resolvedReceiverType?.fullyExpandedType()?.lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return false
        val erased = declared.lookupTag.classId.constructClassLikeType(
            Array(declared.typeArguments.size) { ConeStarProjection },
            isMarkedNullable = true,
        )
        return receiverType.isSubtypeOf(erased, context.session)
    }

    private fun findCall(expression: FirExpression): FirFunctionCall? = when (expression) {
        is FirFunctionCall -> expression
        is FirSafeCallExpression -> expression.selector as? FirFunctionCall
        else -> null
    }

    private fun hasPredicateLambda(call: FirFunctionCall): Boolean =
        call.argumentList.arguments.any { argument ->
            val unwrapped = if (argument is FirWrappedArgumentExpression) argument.expression else argument
            unwrapped is FirAnonymousFunctionExpression && unwrapped.anonymousFunction.isLambda
        }

    private fun isNullLiteral(expression: FirExpression): Boolean =
        expression is FirLiteralExpression && expression.kind == ConstantValueKind.Null
}
