package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Whether a property declared with type [declared] can hold the resource of type [made] itself,
 * so handing the property over hands the resource over. A property holding a value read from the
 * resource (`query(...).count`, `prepareStatement(sql).executeQuery()`) cannot.
 */
context(context: CheckerContext)
fun canHoldResource(declared: ConeKotlinType, made: ConeKotlinType): Boolean = made.isNonNothingSubtypeOf(declared)

/**
 * Whether [declaration], whose source is [source], is handed to other code after its
 * declaration: a resolved read of it that is not the receiver of a member access, an assignment
 * target, a comparison, or an `is` check, or one returned through a scope function that returns
 * its receiver (`return cursor.apply { moveToFirst() }`). Such a read returns, stores, or passes
 * the value on, so its new owner closes it.
 */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
fun isHandedOverAfter(declaration: FirProperty, source: KtSourceElement): Boolean {
    val scope = context.containingDeclarations.lastOrNull { it != declaration.symbol }
        ?: context.containingFileSymbol
        ?: return false
    val symbol = declaration.symbol
    val declarationEnd = source.endOffset
    val notHandedOver = HashSet<FirExpression>()
    var handedOver = false
    fun isRead(expression: FirExpression?): Boolean =
        expression is FirPropertyAccessExpression &&
            (expression.source?.startOffset ?: -1) >= declarationEnd &&
            expression.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() == symbol
    fun exempt(expression: FirExpression?) {
        if (expression == null) return
        notHandedOver += expression
        notHandedOver += unwrapSmartCasts(expression, throughNotNull = true)
    }
    // Local: the scope encloses the property under check (or is its file), so its body is in this compilation.
    scope.fir.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (handedOver) return
            when (element) {
                is FirPropertyAccessExpression -> {
                    if (element !in notHandedOver && isRead(element)) {
                        handedOver = true
                        return
                    }
                }
                is FirReturnExpression -> {
                    if (isRead(receiverReturnedBy(element.result))) {
                        handedOver = true
                        return
                    }
                }
                is FirSafeCallExpression -> exempt(element.receiver)
                is FirVariableAssignment -> assignedAccess(element)?.let { exempt(it) }
                is FirEqualityOperatorCall -> element.argumentList.arguments.forEach(::exempt)
                is FirTypeOperatorCall ->
                    if (element.operation == FirOperation.IS || element.operation == FirOperation.NOT_IS) {
                        element.argumentList.arguments.forEach(::exempt)
                    }
                else -> Unit
            }
            if (element is FirQualifiedAccessExpression) {
                exempt(element.explicitReceiver)
                exempt(element.extensionReceiver)
                exempt(element.dispatchReceiver)
            }
            element.acceptChildren(this)
        }
    })
    return handedOver
}

private val receiverReturningScopeFunctions = setOf("apply", "also", "takeIf", "takeUnless").map {
    CallableId(StandardClassIds.BASE_KOTLIN_PACKAGE, Name.identifier(it))
}.toSet()

// The receiver a chain of `apply`/`also`/`takeIf`/`takeUnless` calls returns, or the expression
// itself when it is not such a call.
private fun receiverReturnedBy(expression: FirExpression): FirExpression {
    var current = unwrapSmartCasts(expression, throughNotNull = true)
    while (true) {
        val call = when (current) {
            is FirSafeCallExpression -> current.selector as? FirFunctionCall
            is FirFunctionCall -> current
            else -> null
        } ?: return current
        if (call.calleeReference.toResolvedCallableSymbol()?.callableId !in receiverReturningScopeFunctions) return current
        val receiver = if (current is FirSafeCallExpression) current.receiver else call.explicitReceiver
        current = unwrapSmartCasts(receiver ?: return current, throughNotNull = true)
    }
}
