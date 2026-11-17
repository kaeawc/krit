package dev.jasonpearson.krit.fir.checkers.privacy

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasEncryptedStorageReceiver
import dev.jasonpearson.krit.fir.support.isSensitiveStorageKey
import dev.jasonpearson.krit.fir.support.isSharedPreferencesMember
import dev.jasonpearson.krit.fir.support.storageKeyLiteral
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId

/**
 * Flags a logger call (`Log.d`, `Timber.e`, ...) whose arguments read a `SharedPreferences` value with `getString`,
 * `getInt` or `getStringSet` and a string-literal key that names a secret (see `isSensitiveStorageKey`). Reported
 * on the preferences read, at any depth of the arguments (concatenations, templates, elvis, safe calls, lambdas),
 * as Go does.
 * Preferences reached through EncryptedSharedPreferences are skipped, also when a `val` of this file holds them.
 *
 * Go takes a logger from the written receiver name `Log` or `Timber`, unless its resolver proves a local value
 * shadows it. This checker resolves the callee instead: a member of a class or object named `Log` or `Timber`
 * (`android.util.Log`, a project wrapper, `Timber.d`, which is `Timber.Forest`, or any `Timber.Tree`), reached under
 * an alias, a star import or a fully qualified name. A local value named `Log` is not a logger.
 */
internal object LogOfSharedPreferenceRead : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LogOfSharedPreferenceRead"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LogOfSharedPreferenceRead)
    }

    private val logMethods = setOf("d", "i", "w", "e", "v", "wtf")
    private val readers = setOf("getString", "getInt", "getStringSet")
    private val loggerNames = setOf("Log", "Timber")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name.asString() !in logMethods) return
        if (!isLogger(callee.unwrapFakeOverrides().callableId?.classId)) return
        // The calls around the element being visited, so a read in `with(prefs) { .. }` finds its receiver.
        val enclosing = ArrayList<FirFunctionCall>()
        val visitor = object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    // `prefs?.getString(..)`: the read is the selector, whose own source is not real; report it on
                    // the whole safe call, as Go reports the call expression.
                    is FirSafeCallExpression ->
                        (element.selector as? FirFunctionCall)?.let { inspect(it, element.source, enclosing) }
                    is FirFunctionCall -> inspect(element, element.source, enclosing)
                    else -> {}
                }
                if (element is FirFunctionCall) enclosing.add(element)
                element.acceptChildren(this)
                if (element is FirFunctionCall) enclosing.removeAt(enclosing.lastIndex)
            }
        }
        for (argument in expression.argumentList.arguments) argument.accept(visitor)
    }

    // android.util.Log, Timber and its Forest and Tree, or any class or object written Log or Timber (as Go takes the
    // receiver by its written name), including a companion nested in one.
    private fun isLogger(owner: ClassId?): Boolean =
        generateSequence(owner) { it.outerClassId }.any { it.shortClassName.asString() in loggerNames }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun inspect(read: FirFunctionCall, anchor: KtSourceElement?, enclosing: List<FirFunctionCall>) {
        val callee = read.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name.asString() !in readers || !isSharedPreferencesMember(read)) return
        val keyParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        val key = read.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == keyParameter }?.key ?: return
        val body = storageKeyLiteral(key) ?: return
        if (!isSensitiveStorageKey(body) || hasEncryptedStorageReceiver(read, enclosing)) return
        val source = anchor?.takeIf { it.kind is KtRealSourceElementKind } ?: return
        report(source, "Logging the value of SharedPreferences key \"$body\". Sensitive data from preferences should not be written to logs.")
    }
}
