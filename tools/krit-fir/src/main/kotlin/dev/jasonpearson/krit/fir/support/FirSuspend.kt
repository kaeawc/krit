package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.InlineStatus
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

private val coroutinesPackage = FqName("kotlin.coroutines")

/** Whether [lambda] was resolved against a `suspend` function type. */
fun hasSuspendFunctionType(lambda: FirAnonymousFunction): Boolean {
    val classId = (lambda.typeRef as? FirResolvedTypeRef)?.coneType?.classId ?: return false
    if (classId.packageFqName != coroutinesPackage) return false
    val name = classId.shortClassName.asString()
    return name.startsWith("SuspendFunction") || name.startsWith("KSuspendFunction")
}

private val inPlaceCrossinline = setOf(
    CallableId(coroutinesPackage, Name.identifier("suspendCoroutine")),
    CallableId(FqName("kotlinx.coroutines"), Name.identifier("suspendCancellableCoroutine")),
)

/**
 * Whether [lambda], the element at [index] of the checker's containing [elements] (outermost first), runs where
 * its call is. An inline lambda does. A crossinline lambda may be stored and run later in another context, so it
 * counts only for the calls that run it before they return.
 */
fun runsInPlace(elements: List<FirElement>, index: Int, lambda: FirAnonymousFunction): Boolean =
    when (lambda.inlineStatus) {
        InlineStatus.Inline -> true
        InlineStatus.CrossInline -> {
            val owner = lambdaOwner(elements, index, lambda)?.calleeReference?.toResolvedCallableSymbol()
            owner?.callableId in inPlaceCrossinline
        }
        else -> false
    }

private val coroutineContext = CallableId(FqName("kotlin.coroutines"), Name.identifier("coroutineContext"))

/**
 * Whether [body] suspends its own coroutine: it calls a resolved suspend function (an invoked suspend function
 * value and a `for` over a channel included, since the compiler desugars the loop to calls) or reads
 * `kotlin.coroutines.coroutineContext`. A lambda counts only when it runs in place (passed to an inline
 * function and not itself a suspend lambda). Local functions, classes and objects own their scope.
 */
fun suspendsInPlace(body: FirElement): Boolean {
    val finder = SuspendFinder()
    body.accept(finder)
    return finder.found
}

private class SuspendFinder : FirVisitorVoid() {
    var found = false

    override fun visitElement(element: FirElement) {
        if (!found) element.acceptChildren(this)
    }

    override fun visitFunctionCall(functionCall: FirFunctionCall) {
        val callee = functionCall.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol
        if (callee?.isSuspend == true) {
            found = true
            return
        }
        visitElement(functionCall)
    }

    override fun visitImplicitInvokeCall(implicitInvokeCall: FirImplicitInvokeCall) {
        visitFunctionCall(implicitInvokeCall)
    }

    override fun visitPropertyAccessExpression(propertyAccessExpression: FirPropertyAccessExpression) {
        if (propertyAccessExpression.calleeReference.toResolvedCallableSymbol()?.callableId == coroutineContext) {
            found = true
            return
        }
        visitElement(propertyAccessExpression)
    }

    override fun visitAnonymousFunction(anonymousFunction: FirAnonymousFunction) {
        val inPlace = anonymousFunction.inlineStatus == InlineStatus.Inline ||
            anonymousFunction.inlineStatus == InlineStatus.CrossInline
        if (inPlace && !hasSuspendFunctionType(anonymousFunction)) visitElement(anonymousFunction)
    }

    override fun visitNamedFunction(namedFunction: FirNamedFunction) {
        // A local function owns its scope: its calls do not suspend the outer one.
    }

    override fun visitRegularClass(regularClass: FirRegularClass) {
        // A local class owns its scope.
    }

    override fun visitAnonymousObject(anonymousObject: FirAnonymousObject) {
        // An object literal owns its scope.
    }
}
