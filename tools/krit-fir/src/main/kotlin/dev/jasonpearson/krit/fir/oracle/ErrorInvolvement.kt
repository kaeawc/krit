package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirErrorExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.FirErrorNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.resolvedType

/**
 * Whether the type of an expression derives from an error type: a class that is not on the compilation's classpath
 * (a module whose build output is missing) or a reference that did not resolve.
 *
 * The compiler treats an error type as compatible with every type and as non-null, and gives a member looked up on
 * it the first extension of that name. Verdicts that rest on such a type, such as a "useless" elvis or code that
 * follows a `Nothing`, describe the compiler's guess rather than the program, so the collector drops them.
 */
internal object ErrorInvolvement {

    /** Whether [expression] has an error type, is unresolved, or takes its type from an operand that does. */
    fun involvesError(expression: FirExpression?): Boolean = when (expression) {
        null -> false
        is FirErrorExpression -> true
        is FirSmartCastExpression -> typeIsError(expression) || involvesError(expression.originalExpression)
        is FirSafeCallExpression -> involvesError(expression.receiver) || involvesError(expression.selector as? FirExpression)
        is FirCheckedSafeCallSubject -> involvesError(expression.originalReceiverRef.value)
        is FirCheckNotNullCall -> involvesError(expression.argumentList.arguments.firstOrNull())
        is FirElvisExpression -> involvesError(expression.lhs) || involvesError(expression.rhs)
        is FirWhenExpression -> expression.branches.any { involvesError(resultOf(it.result)) }
        is FirTryExpression -> involvesError(resultOf(expression.tryBlock)) ||
            expression.catches.any { involvesError(resultOf(it.block)) }
        is FirBlock -> involvesError(resultOf(expression))
        is FirWrappedArgumentExpression -> involvesError(expression.expression)
        is FirTypeOperatorCall -> typeIsError(expression)
        is FirQualifiedAccessExpression -> qualifiedAccessInvolvesError(expression)
        else -> typeIsError(expression)
    }

    /**
     * The expression that gives a block statement its error type: the statement itself, the initializer of a local,
     * or the assigned value. Null when the statement does not involve an error type.
     */
    fun errorExpressionOf(statement: FirStatement): FirExpression? = when (statement) {
        is FirProperty -> statement.initializer?.takeIf { statement.isLocal && involvesError(it) }
        is FirVariableAssignment -> statement.rValue.takeIf { involvesError(it) }
        is FirExpression -> statement.takeIf { involvesError(it) }
        else -> null
    }

    /**
     * Whether the callee of [call] was looked up on a receiver that involves an error. An error receiver matches the
     * receiver of every extension, so the compiler binds the first extension of that name: the callee, its return
     * type and its deprecation describe a declaration the program may not call.
     */
    fun calleeResolvedThroughError(call: FirQualifiedAccessExpression): Boolean =
        involvesError(call.explicitReceiver) ||
            involvesError(call.dispatchReceiver) ||
            involvesError(call.extensionReceiver)

    fun typeIsError(expression: FirExpression): Boolean =
        runCatching { expression.resolvedType }.getOrNull() is ConeErrorType

    @OptIn(SymbolInternals::class)
    private fun qualifiedAccessInvolvesError(access: FirQualifiedAccessExpression): Boolean {
        if (typeIsError(access) || access.calleeReference is FirErrorNamedReference) return true
        if (involvesError(access.explicitReceiver)) return true
        if (access !is FirFunctionCall) return false
        // A generic callee takes its return type from its arguments, so an error argument can reach the result.
        val callee = access.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        return callee.fir.typeParameters.isNotEmpty() && access.argumentList.arguments.any { involvesError(it) }
    }

    /** The expression a branch or block evaluates to, or null when it ends in a statement that is not an expression. */
    private fun resultOf(block: FirBlock?): FirExpression? = block?.statements?.lastOrNull() as? FirExpression
}
