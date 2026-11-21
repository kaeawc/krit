package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.checkers.delegateInCheckedFile
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

private val PLAIN_FILE = classIdOf("java.io", "File")
private val PLAIN_PATH = classIdOf("java.nio.file", "Path")
private val ENCRYPTED_STORAGE = listOf(
    classIdOf("androidx.security.crypto", "EncryptedSharedPreferences"),
    classIdOf("androidx.security.crypto", "EncryptedFile"),
)

context(context: CheckerContext)
fun isPlainFileWrite(call: FirFunctionCall): Boolean {
    val id = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
    if (id.callableName.asString() !in setOf("writeText", "writeBytes")) return false
    val isFileMember = id.classId == PLAIN_FILE
    val hasNoClass = id.classId == null
    val packageName = id.packageName.asString()
    val isKotlinIoFileWrite = hasNoClass && packageName == "kotlin.io" && isMemberOf(call, PLAIN_FILE)
    val receiver = call.explicitReceiver ?: call.extensionReceiver
    val toPath = receiver?.let { unwrapSmartCasts(unwrapArgument(it), throughNotNull = true) as? FirFunctionCall }
    val pathId = toPath?.calleeReference?.toResolvedCallableSymbol()?.callableId
    val isFileToPath = pathId?.let {
        it.classId == PLAIN_FILE && it.callableName.asString() == "toPath"
    } == true
    val isKotlinIoPathWrite = hasNoClass && packageName == "kotlin.io.path" &&
        isMemberOf(call, PLAIN_PATH) && isFileToPath
    return isFileMember || isKotlinIoFileWrite || isKotlinIoPathWrite
}

/**
 * Whether [call]'s receiver chain is encrypted storage: the receiver, or any receiver or qualifier it is built
 * from, has type `EncryptedSharedPreferences` or `EncryptedFile` (or a subtype), or is reached through one of
 * them (`EncryptedSharedPreferences.create(...).edit()`). The receiver is the implicit one when the call has no
 * explicit receiver, so `prefs.edit { putString(...) }` is encrypted when `prefs` is. A `val` declared in the
 * file under check (local, member or top-level) is followed to its initializer or `by lazy { }` result, so
 * `private val prefs = EncryptedSharedPreferences.create(...)` makes `prefs.edit()` encrypted.
 */
context(context: CheckerContext)
fun hasEncryptedStorageReceiver(call: FirFunctionCall, enclosingCalls: List<FirFunctionCall> = emptyList()): Boolean =
    isEncryptedChain(call.explicitReceiver ?: implicitStorageReceiver(call, enclosingCalls), depth = 0)

context(context: CheckerContext)
private fun implicitStorageReceiver(call: FirFunctionCall, enclosingCalls: List<FirFunctionCall>): FirExpression? {
    val receiver = (call.dispatchReceiver ?: call.extensionReceiver)?.unwrapSmartcastExpression()
        as? FirThisReceiverExpression ?: return null
    val parameter = receiver.calleeReference.boundSymbol as? FirReceiverParameterSymbol ?: return null
    val lambda = parameter.containingDeclarationSymbol as? FirAnonymousFunctionSymbol ?: return null
    val passesLambda = { argument: FirExpression ->
        (unwrapArgument(argument) as? FirAnonymousFunctionExpression)?.anonymousFunction?.symbol == lambda
    }
    val takesLambda = { candidate: Any? ->
        candidate is FirFunctionCall && candidate.argumentList.arguments.any(passesLambda)
    }
    val owner = (enclosingCalls.lastOrNull(takesLambda) ?: context.callsOrAssignments.lastOrNull(takesLambda))
        as? FirFunctionCall ?: return null
    if (owner.calleeReference.toResolvedCallableSymbol()?.callableId == WITH) {
        return owner.argumentList.arguments.firstOrNull { !passesLambda(it) }
    }
    return owner.explicitReceiver
}

private val WITH = CallableId(FqName("kotlin"), Name.identifier("with"))

context(context: CheckerContext)
private fun isEncryptedChain(start: FirExpression?, depth: Int): Boolean {
    var current = start
    while (current != null) {
        val expression = current.unwrapSmartcastExpression()
        if (isEncryptedType(expression) ||
            (expression as? FirQualifiedAccessExpression)
                ?.calleeReference?.toResolvedCallableSymbol()?.callableId?.classId
                ?.let { classId -> ENCRYPTED_STORAGE.any { classId == it || classId.outerClassId == it } } == true
        ) {
            return true
        }
        val property = (expression as? FirPropertyAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
        if (property is FirPropertySymbol && followsEncryptedProperty(property, depth)) return true
        current = (expression as? FirQualifiedAccessExpression)?.explicitReceiver
    }
    return false
}

context(context: CheckerContext)
private fun followsEncryptedProperty(property: FirPropertySymbol, depth: Int): Boolean =
    !property.isVar && depth < MAX_LOCAL_DEPTH &&
        storedValues(property).any { value -> isEncryptedChain(value, depth + 1) }

context(context: CheckerContext)
private fun storedValues(property: FirPropertySymbol): List<FirExpression> {
    val lazy = property.delegateInCheckedFile() as? FirFunctionCall
    val lazyResult = lazy?.takeIf { it.calleeReference.toResolvedCallableSymbol()?.callableId == LAZY }
        ?.argumentList?.arguments?.lastOrNull()
        ?.let { (unwrapArgument(it) as? FirAnonymousFunctionExpression)?.anonymousFunction?.body?.statements?.lastOrNull() }
        ?.let { if (it is FirReturnExpression) it.result else it as? FirExpression }
    return listOfNotNull(property.initializerInCheckedFile(), lazyResult)
}

private val LAZY = CallableId(FqName("kotlin"), Name.identifier("lazy"))

private const val MAX_LOCAL_DEPTH = 4

context(context: CheckerContext)
private fun isEncryptedType(expression: FirExpression): Boolean {
    if (expression is FirResolvedQualifier) return expression.qualifierSymbol?.classId in ENCRYPTED_STORAGE
    val type = expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType ?: return false
    val symbol = type.lookupTag.toClassSymbol(context.session) ?: return false
    return ENCRYPTED_STORAGE.any { isOrInheritsFrom(symbol, it, context.session, lookupInterfaces = true) }
}
