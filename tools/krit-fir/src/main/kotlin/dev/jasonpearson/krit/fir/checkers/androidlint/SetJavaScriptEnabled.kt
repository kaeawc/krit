package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirVariableAssignmentChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.expressions.unwrapLValue
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags enabling JavaScript on a `WebSettings`: a `setJavaScriptEnabled(true)`
 * call or a `javaScriptEnabled = true` assignment of the (synthetic or Kotlin)
 * property over that setter.
 *
 * Like the Go rule, the value must be the literal `true` (a constant, a
 * variable, or `!false` is not a finding), and the finding sits on the first
 * line of the call or assignment. For a call, the value is the argument bound
 * to the first parameter, as Go reads the first argument.
 *
 * The owner comes from the resolved symbol: the member must be declared on a
 * class whose simple name is `WebSettings`, in any package, or on a subtype of
 * one (including local and anonymous classes), or be an extension on one of
 * those types. Go matches the receiver type by the simple name `WebSettings`
 * too, so this covers `android.webkit.WebSettings` and third-party engines that
 * reuse its name and setter, such as Tencent X5's
 * `com.tencent.smtt.sdk.WebSettings`, where enabling JavaScript is the same
 * XSS surface. Go reports a user overload such as
 * `fun WebSettings.setJavaScriptEnabled(enabled: Boolean, log: Boolean)`
 * through its receiver type, and so does this checker.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Go also falls back to a `WebView`-typed parameter chain through
 *   `settings`, so it reports a `javaScriptEnabled` property of an unrelated
 *   class reached through `settings` (`view.settings.extra.javaScriptEnabled`).
 *   That code does not enable JavaScript on any web settings, so this checker
 *   does not report it.
 * - Resolution sees receivers Go's source inference cannot type (locals, class
 *   properties, `getSettings()`, `!!`, parenthesized and elvis receivers,
 *   anonymous subclasses, implicit receivers such as
 *   `settings.apply { javaScriptEnabled = true }`, and initializers).
 * - Go needs the value to be a bare `boolean_literal` node in the first
 *   unlabeled argument. A parenthesized, annotated, or labeled `true`, and a
 *   named `flag = true` argument, are still the literal `true` here.
 */
internal object SetJavaScriptEnabled : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SetJavaScriptEnabled"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SetJavaScriptEnabled)
        override val variableAssignmentCheckers = setOf(AssignmentChecker)
    }

    private const val MESSAGE = "Using setJavaScriptEnabled(true). Review for XSS vulnerabilities."
    private val WEB_SETTINGS = Name.identifier("WebSettings")
    private val SETTER = Name.identifier("setJavaScriptEnabled")
    private val PROPERTY = Name.identifier("javaScriptEnabled")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name != SETTER || !isWebSettingsMember(callee)) return
        val firstParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        val value = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == firstParameter }
            ?.key
            ?: return
        if (!isTrueLiteral(value)) return
        report(expression.source, MESSAGE)
    }

    private object AssignmentChecker : FirVariableAssignmentChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirVariableAssignment) {
            val target = expression.unwrapLValue() ?: return
            val symbol = target.calleeReference.toResolvedCallableSymbol() ?: return
            if (symbol.name != PROPERTY || !isWebSettingsMember(symbol)) return
            if (!isTrueLiteral(expression.rValue)) return
            report(expression.source, MESSAGE)
        }
    }

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; looking one of those up by class id throws.
    // Only class ids already in hand are compared, never looked up.
    context(context: CheckerContext)
    private fun isWebSettingsMember(symbol: FirCallableSymbol<*>): Boolean {
        val owner = symbol.getContainingClassSymbol()
        if (owner != null && isWebSettingsClass(owner)) return true
        val receiver = symbol.resolvedReceiverType?.toClassLikeSymbol(context.session) ?: return false
        return isWebSettingsClass(receiver)
    }

    context(context: CheckerContext)
    private fun isWebSettingsClass(symbol: FirClassLikeSymbol<*>): Boolean =
        symbol.classId.shortClassName == WEB_SETTINGS ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId.shortClassName == WEB_SETTINGS }

    // FIR drops the parentheses, annotation, or label around a literal, so a
    // wrapped `true` is still a Boolean literal here.
    private fun isTrueLiteral(expression: FirExpression): Boolean {
        val literal = expression as? FirLiteralExpression ?: return false
        return literal.kind == ConstantValueKind.Boolean && literal.value == true
    }
}
