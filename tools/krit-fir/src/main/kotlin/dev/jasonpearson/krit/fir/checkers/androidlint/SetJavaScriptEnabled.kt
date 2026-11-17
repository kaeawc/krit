package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.inheritsFrom
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirVariableAssignmentChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.expressions.unwrapLValue
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags enabling JavaScript on a `WebSettings`: a `setJavaScriptEnabled(true)`
 * call or a `javaScriptEnabled = true` assignment of the (synthetic or Kotlin)
 * property over that setter.
 *
 * Like the Go rule, the value must be `true` as Lint's ConstantEvaluator folds
 * it: the literal, `!` of a known value, or a val or constant whose
 * initializer is one (a parameter, a var, or a call is not a finding), and the
 * finding sits on the first line of the call or assignment. For a call, the
 * value is the argument bound to the first parameter, as Go reads the first
 * argument.
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
 *   does not report it. Without types, Go also takes a receiver named like
 *   web settings (`settings`, `webSettings`, a `.settings` chain) in a file
 *   that uses the WebView API; this checker decides by the receiver's
 *   resolved type instead.
 * - Resolution sees receivers Go can neither type nor recognize by name
 *   (`!!`, parenthesized and elvis receivers, WebSettings subclasses and
 *   anonymous subclasses, extension receivers). Go recognizes a `settings`
 *   chain, `getSettings()`, a name declared as WebSettings, and the implicit
 *   receiver of a scope function on one of those.
 * - Go reads the value from the first unlabeled argument and follows only a
 *   bare name declared in the same file. An annotated or labeled `true`, a
 *   named `flag = true` argument, and a qualified constant
 *   (`Config.SCRIPTS_ON`) are still `true` here.
 */
internal object SetJavaScriptEnabled : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SetJavaScriptEnabled"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SetJavaScriptEnabled)
        override val variableAssignmentCheckers = setOf(AssignmentChecker)
    }

    private const val MESSAGE = "Using `setJavaScriptEnabled` can introduce XSS vulnerabilities into your application, review carefully"
    private val WEB_SETTINGS = Name.identifier("WebSettings")
    private val SETTER = Name.identifier("setJavaScriptEnabled")
    private val PROPERTY = Name.identifier("javaScriptEnabled")
    private val booleanNot = CallableId(StandardClassIds.Boolean, Name.identifier("not"))

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
        if (!isTrue(value)) return
        report(expression.source, MESSAGE)
    }

    private object AssignmentChecker : FirVariableAssignmentChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirVariableAssignment) {
            val target = expression.unwrapLValue() ?: return
            val symbol = target.calleeReference.toResolvedCallableSymbol() ?: return
            if (symbol.name != PROPERTY || !isWebSettingsMember(symbol)) return
            if (!isTrue(expression.rValue)) return
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
            inheritsFrom(symbol, context.session, lookupInterfaces = true) { it.shortClassName == WEB_SETTINGS }

    // Go's lintEvalBool, after Lint's ConstantEvaluator: the literal `true`, `!` of a known value, or a val or
    // constant whose initializer is one. FIR drops the parentheses, annotation, or label around a literal, so a
    // wrapped `true` is still a Boolean literal here.
    context(context: CheckerContext)
    private fun isTrue(expression: FirExpression): Boolean = booleanValue(expression, HashSet()) == true

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun booleanValue(expression: FirExpression, following: MutableSet<FirCallableSymbol<*>>): Boolean? {
        return when (val value = unwrapSmartCasts(unwrapArgument(expression))) {
            is FirLiteralExpression -> (value.value as? Boolean)?.takeIf { value.kind == ConstantValueKind.Boolean }
            is FirFunctionCall -> {
                if (value.calleeReference.toResolvedCallableSymbol()?.callableId != booleanNot) return null
                value.explicitReceiver?.let { booleanValue(it, following) }?.not()
            }
            is FirQualifiedAccessExpression -> {
                val symbol = value.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() ?: return null
                if (!following.add(symbol)) return null
                val initializer = when (symbol) {
                    // A const keeps its value in a class file; a val is followed only in the file under check.
                    is FirPropertySymbol -> symbol.takeIf {
                        it.isVal && !it.hasDelegate && it.getterSymbol?.isDefault != false
                    }?.followableInitializer()
                    is FirFieldSymbol -> symbol.fir.takeIf { it.isVal }?.initializer
                    else -> null
                }
                initializer?.let { booleanValue(it, following) }
            }
            else -> null
        }
    }
}
