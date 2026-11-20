package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.checkers.delegateInCheckedFile
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

private val SENSITIVE_KEY_WORDS = Regex(
    "(^|_)(tokens?|secrets?|passwords?|passwd|pin|pins|auth|oauth|credentials?|private_?keys?|api_?keys?)(_|$)",
    RegexOption.IGNORE_CASE,
)
private val CAMEL_BOUNDARY = Regex("([a-z0-9])([A-Z])|([A-Z]+)([A-Z][a-z])")
private val SENSITIVE_FILE_WORDS = Regex(
    "(^|_)(credentials?|tokens?|secrets?|auth|oauth|passwords?|passwd|private_?keys?)(_|$)",
    RegexOption.IGNORE_CASE,
)
private val NON_WORD_RUN = Regex("[^A-Za-z0-9]+")

// Words that mark a key as metadata about a secret (its expiry, count or flag), not the secret.
private val BENIGN_KEY_WORDS = setOf(
    "expires", "expiry", "expiration", "expired", "expire", "time", "timestamp", "at", "count", "counter",
    "attempts", "enabled", "disabled", "visible", "shown", "hint", "type", "state", "status", "version",
    "length", "label", "has", "is",
)

private val SHARED_PREFERENCES = classIdOf("android.content", "SharedPreferences")
private val PLAIN_FILE = classIdOf("java.io", "File")
private val PLAIN_PATH = classIdOf("java.nio.file", "Path")
private val SHARED_PREFERENCES_EDITOR = SHARED_PREFERENCES.createNestedClassId(Name.identifier("Editor"))
private val ENCRYPTED_STORAGE = listOf(
    classIdOf("androidx.security.crypto", "EncryptedSharedPreferences"),
    classIdOf("androidx.security.crypto", "EncryptedFile"),
)

// Splits a camelCase or punctuated name into lowercase-able snake_case words.
private fun snakeCase(name: String): String =
    NON_WORD_RUN.replace(CAMEL_BOUNDARY.replace(name, "\$1\$3_\$2\$4"), "_")

/**
 * Whether a SharedPreferences key names a secret value, as Go's `sensitiveStorageKey` decides: a whole
 * snake_case word such as `token` or `api_key` (also after splitting camelCase), and no word that marks the key as
 * metadata about the secret (`token_expires`, `has_pin`).
 */
fun isSensitiveStorageKey(key: String): Boolean {
    val snake = snakeCase(key)
    if (!SENSITIVE_KEY_WORDS.containsMatchIn(key) && !SENSITIVE_KEY_WORDS.containsMatchIn(snake)) return false
    return NON_WORD_RUN.split(snake.lowercase()).none { it in BENIGN_KEY_WORDS }
}

fun isSensitiveFileName(path: String): Boolean =
    SENSITIVE_FILE_WORDS.containsMatchIn(path) || SENSITIVE_FILE_WORDS.containsMatchIn(snakeCase(path))

context(context: CheckerContext)
fun isPlainFileWrite(call: FirFunctionCall): Boolean {
    val callee = call.calleeReference.toResolvedCallableSymbol() ?: return false
    val id = callee.callableId ?: return false
    if (id.callableName.asString() !in setOf("writeText", "writeBytes")) return false
    if (id.classId == PLAIN_FILE) return true
    if (id.classId != null) return false
    if (id.packageName.asString() == "kotlin.io") return isMemberOf(call, PLAIN_FILE)
    if (id.packageName.asString() != "kotlin.io.path" || !isMemberOf(call, PLAIN_PATH)) return false
    val receiver = call.explicitReceiver ?: call.extensionReceiver ?: return false
    val toPath = unwrapSmartCasts(unwrapArgument(receiver), throughNotNull = true) as? FirFunctionCall ?: return false
    val toPathId = toPath.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
    return toPathId.classId == PLAIN_FILE && toPathId.callableName.asString() == "toPath"
}

fun plainFileReceiverLiteral(call: FirFunctionCall): String? {
    val receiver = call.explicitReceiver ?: call.extensionReceiver ?: call.dispatchReceiver ?: return null
    var file: FirFunctionCall? = null
    val visitor = object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (file != null) return
            if (element is FirFunctionCall) {
                val constructor = element.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol
                if (constructor?.callableId?.classId == PLAIN_FILE) {
                    file = element
                    return
                }
            }
            element.acceptChildren(this)
        }
    }
    unwrapSmartCasts(unwrapArgument(receiver), throughNotNull = true).accept(visitor)
    for (argument in file?.argumentList?.arguments.orEmpty()) {
        val value = unwrapArgument(argument)
        if (value is FirLiteralExpression || value is FirStringConcatenationCall) {
            plainFileNameLiteral(value)?.let { return it }
        }
    }
    return null
}

private fun plainFileNameLiteral(expression: FirExpression): String? {
    if (expression !is FirLiteralExpression && expression !is FirStringConcatenationCall) return null
    val source = expression.source?.takeIf { it.kind is KtRealSourceElementKind } ?: return null
    val literal = source.lighterASTNode
    if (literal.tokenType != KtNodeTypes.STRING_TEMPLATE) return null
    return lightChildren(source, literal)
        .filter {
            it.tokenType == KtNodeTypes.LITERAL_STRING_TEMPLATE_ENTRY ||
                it.tokenType == KtNodeTypes.ESCAPE_STRING_TEMPLATE_ENTRY
        }
        .joinToString("") { lightText(source, it) }
}

/**
 * SharedPreferences key text of [argument]: a plain literal uses its FIR value, while a template keeps its
 * `$name` source text, matching the template reading of the SharedPreferences Go rules. A `+` chain that starts
 * and ends with a literal (`"user_" + id + "_token"`) is read the same way, from its first quote to its last,
 * as Go does. Null for any other expression, including a constant.
 */
fun storageKeyLiteral(argument: FirExpression): String? {
    val expression = unwrapArgument(argument)
    if (expression is FirLiteralExpression) {
        return (expression.value as? String)?.takeIf { expression.kind == ConstantValueKind.String }
    }
    if (expression !is FirStringConcatenationCall && !isStringPlus(expression)) return null
    val source = expression.source?.takeIf { it.kind is KtRealSourceElementKind } ?: return null
    val text = lightText(source, source.lighterASTNode)
    return when {
        text.length >= 6 && text.startsWith("\"\"\"") && text.endsWith("\"\"\"") -> text.substring(3, text.length - 3)
        text.length >= 2 && text.startsWith("\"") && text.endsWith("\"") -> text.substring(1, text.length - 1)
        else -> null
    }
}

private val STRING_PLUS = CallableId(ClassId(FqName("kotlin"), Name.identifier("String")), Name.identifier("plus"))

private fun isStringPlus(expression: FirExpression): Boolean =
    expression is FirFunctionCall && expression.calleeReference.toResolvedCallableSymbol()?.callableId == STRING_PLUS

/**
 * Whether [call] resolves to a member of `android.content.SharedPreferences` or one of its subtypes, or to an
 * extension function on one (`fun SharedPreferences.getString(key: String)`), as Go matches the receiver type.
 */
context(context: CheckerContext)
fun isSharedPreferencesMember(call: FirFunctionCall): Boolean = isMemberOf(call, SHARED_PREFERENCES)

/** [isSharedPreferencesMember] for `android.content.SharedPreferences.Editor`. */
context(context: CheckerContext)
fun isSharedPreferencesEditorMember(call: FirFunctionCall): Boolean = isMemberOf(call, SHARED_PREFERENCES_EDITOR)

context(context: CheckerContext)
private fun isMemberOf(call: FirFunctionCall, target: ClassId): Boolean {
    val callee = call.calleeReference.toResolvedCallableSymbol() ?: return false
    val owner = callee.getContainingClassSymbol()
        ?: return callee.resolvedReceiverType?.isNonNothingSubtypeOf(target.nullableClassType()) == true
    return isOrInheritsFrom(owner, target, context.session, lookupInterfaces = true)
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

// `prefs.edit { putString(..) }`, `with(prefs.edit()) { .. }`: the implicit receiver is a lambda's, so the chain
// continues at what the call taking the lambda is made on (its receiver, or the other argument of `with`). That
// call is one of [enclosingCalls] (innermost last), for a checker that walks into a call's arguments itself, or
// one the checker context encloses.
context(context: CheckerContext)
private fun implicitStorageReceiver(call: FirFunctionCall, enclosingCalls: List<FirFunctionCall>): FirExpression? {
    val receiver = (call.dispatchReceiver ?: call.extensionReceiver)?.unwrapSmartcastExpression()
        as? FirThisReceiverExpression ?: return null
    val parameter = receiver.calleeReference.boundSymbol as? FirReceiverParameterSymbol ?: return null
    val lambda = parameter.containingDeclarationSymbol as? FirAnonymousFunctionSymbol ?: return null
    val passesLambda = { argument: FirExpression ->
        (unwrapArgument(argument) as? FirAnonymousFunctionExpression)?.anonymousFunction?.symbol == lambda
    }
    val takesLambda = { candidate: Any? -> candidate is FirFunctionCall && candidate.argumentList.arguments.any(passesLambda) }
    val owner = (enclosingCalls.lastOrNull(takesLambda) ?: context.callsOrAssignments.lastOrNull(takesLambda))
        as? FirFunctionCall ?: return null
    if (owner.calleeReference.toResolvedCallableSymbol()?.callableId == WITH) {
        return owner.argumentList.arguments.firstOrNull { !passesLambda(it) }
    }
    return owner.explicitReceiver
}

private val WITH = CallableId(FqName("kotlin"), Name.identifier("with"))

// Only a declaration of the file under check is followed: another file's initializer is unknown on an
// incremental slice.
context(context: CheckerContext)
private fun isEncryptedChain(start: FirExpression?, depth: Int): Boolean {
    var current = start
    while (current != null) {
        val expression = current.unwrapSmartcastExpression()
        if (isEncryptedType(expression) || isEncryptedOwner(expression)) return true
        val property = (expression as? FirPropertyAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
        if (property is FirPropertySymbol && !property.isVar && depth < MAX_LOCAL_DEPTH &&
            storedValues(property).any { isEncryptedChain(it, depth + 1) }
        ) {
            return true
        }
        current = (expression as? FirQualifiedAccessExpression)?.explicitReceiver
    }
    return false
}

// The initializer of [property], or the result of its `by lazy { }` block.
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

private fun isEncryptedOwner(expression: FirExpression): Boolean {
    val access = expression as? FirQualifiedAccessExpression ?: return false
    val classId: ClassId = access.calleeReference.toResolvedCallableSymbol()?.callableId?.classId ?: return false
    return ENCRYPTED_STORAGE.any { classId == it || classId.outerClassId == it }
}
