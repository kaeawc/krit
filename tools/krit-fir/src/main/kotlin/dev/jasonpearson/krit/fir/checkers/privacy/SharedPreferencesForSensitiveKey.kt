package dev.jasonpearson.krit.fir.checkers.privacy

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasEncryptedStorageReceiver
import dev.jasonpearson.krit.fir.support.isSensitiveStorageKey
import dev.jasonpearson.krit.fir.support.isSharedPreferencesEditorMember
import dev.jasonpearson.krit.fir.support.storageKeyLiteral
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol

/**
 * Flags `putString`, `putInt` and `putStringSet` on a `SharedPreferences.Editor` whose key is a string literal that
 * names a secret (`auth_token`, `apiKey`; see `isSensitiveStorageKey`), reported on the key argument. Sensitive
 * values belong in EncryptedSharedPreferences or the Keystore, so an editor reached through either is skipped.
 *
 * Go matches the call by name and proves the owner only from names its resolver knows or a same-file declaration.
 * This checker resolves the callee, so an editor held in a property, returned by a call, or reached through an
 * implicit receiver counts, and a same-named method of another class never does.
 */
internal object SharedPreferencesForSensitiveKey : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SharedPreferencesForSensitiveKey"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SharedPreferencesForSensitiveKey)
    }

    private val putters = setOf("putString", "putInt", "putStringSet")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name.asString() !in putters || !isSharedPreferencesEditorMember(expression)) return
        if (hasEncryptedStorageReceiver(expression)) return
        val keyParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        val key = expression.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == keyParameter }?.key
            ?: return
        val body = storageKeyLiteral(key) ?: return
        if (!isSensitiveStorageKey(body)) return
        report(
            unwrapArgument(key).source,
            "SharedPreferences key \"$body\" looks sensitive. Use EncryptedSharedPreferences or the Android Keystore for sensitive data.",
        )
    }
}
