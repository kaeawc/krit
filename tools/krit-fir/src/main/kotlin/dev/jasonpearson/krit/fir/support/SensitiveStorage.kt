package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

// Shared by the SharedPreferences rules that decide whether a preference key names a secret
// (SharedPreferencesForSensitiveKey, LogOfSharedPreferenceRead).

private val SENSITIVE_KEY_WORDS = Regex(
    "(^|_)(tokens?|secrets?|passwords?|passwd|pin|pins|auth|oauth|credentials?|private_?keys?|api_?keys?)(_|$)",
    RegexOption.IGNORE_CASE,
)
private val CAMEL_BOUNDARY = Regex("([a-z0-9])([A-Z])|([A-Z]+)([A-Z][a-z])")
private val NON_WORD_RUN = Regex("[^A-Za-z0-9]+")

// Words that mark a key as metadata about a secret (its expiry, count or flag), not the secret.
private val BENIGN_KEY_WORDS = setOf(
    "expires", "expiry", "expiration", "expired", "expire", "time", "timestamp", "at", "count", "counter",
    "attempts", "enabled", "disabled", "visible", "shown", "hint", "type", "state", "status", "version",
    "length", "label", "has", "is",
)

private val SHARED_PREFERENCES = classIdOf("android.content", "SharedPreferences")
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

/**
 * The key text of [argument] when it is a string literal or template, as written between the quotes (templates
 * keep their `$name` text, as Go reads the source). Null for any other expression, including a constant.
 */
fun storageKeyLiteral(argument: FirExpression): String? {
    val expression = unwrapArgument(argument)
    if (expression is FirLiteralExpression) {
        return (expression.value as? String)?.takeIf { expression.kind == ConstantValueKind.String }
    }
    if (expression !is FirStringConcatenationCall) return null
    val source = expression.source?.takeIf { it.kind is KtRealSourceElementKind } ?: return null
    val text = lightText(source, source.lighterASTNode)
    return when {
        text.length >= 6 && text.startsWith("\"\"\"") && text.endsWith("\"\"\"") -> text.substring(3, text.length - 3)
        text.length >= 2 && text.startsWith("\"") && text.endsWith("\"") -> text.substring(1, text.length - 1)
        else -> null
    }
}

/** Whether [call] resolves to a member of `android.content.SharedPreferences` or one of its subtypes. */
context(context: CheckerContext)
fun isSharedPreferencesMember(call: FirFunctionCall): Boolean = isMemberOf(call, SHARED_PREFERENCES)

/** Whether [call] resolves to a member of `android.content.SharedPreferences.Editor` or one of its subtypes. */
context(context: CheckerContext)
fun isSharedPreferencesEditorMember(call: FirFunctionCall): Boolean = isMemberOf(call, SHARED_PREFERENCES_EDITOR)

context(context: CheckerContext)
private fun isMemberOf(call: FirFunctionCall, target: ClassId): Boolean {
    val owner = call.calleeReference.toResolvedCallableSymbol()?.getContainingClassSymbol() ?: return false
    return isOrInheritsFrom(owner, target, context.session, lookupInterfaces = true)
}

/**
 * Whether [call]'s receiver chain is encrypted storage: the receiver, or any receiver or qualifier it is built
 * from, has type `EncryptedSharedPreferences` or `EncryptedFile` (or a subtype), or is reached through one of
 * them (`EncryptedSharedPreferences.create(...).edit()`). A local `val` is followed to its initializer, so
 * `val prefs = EncryptedSharedPreferences.create(...)` makes `prefs.edit()` encrypted.
 */
context(context: CheckerContext)
fun hasEncryptedStorageReceiver(call: FirFunctionCall): Boolean = isEncryptedChain(call.explicitReceiver, depth = 0)

// A local initializer is the only read of another declaration; it is in the file under check.
context(context: CheckerContext)
private fun isEncryptedChain(start: FirExpression?, depth: Int): Boolean {
    var current = start
    while (current != null) {
        val expression = current.unwrapSmartcastExpression()
        if (isEncryptedType(expression) || isEncryptedOwner(expression)) return true
        val local = (expression as? FirPropertyAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
        if (local is FirPropertySymbol && local.isLocal && !local.isVar && depth < MAX_LOCAL_DEPTH &&
            isEncryptedChain(local.resolvedInitializer, depth + 1)
        ) {
            return true
        }
        current = (expression as? FirQualifiedAccessExpression)?.explicitReceiver
    }
    return false
}

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
