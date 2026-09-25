package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtLightSourceElement
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtPsiSourceElement
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirVariableAssignmentChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirNamedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.unwrapLValue
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KtParenthesizedExpression
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags enabling JavaScript on an `android.webkit.WebSettings`: a
 * `setJavaScriptEnabled(true)` call or a `javaScriptEnabled = true` assignment
 * of the synthetic property over that setter.
 *
 * Matches the Go rule: the value must be the literal `true` written directly
 * (Go reads a bare `boolean_literal`, so a parenthesized `(true)`, a constant,
 * or a variable is not a finding), and the finding sits on the first line of
 * the call or assignment.
 *
 * The owner comes from the resolved symbol: the member must be declared on
 * `WebSettings` or on a subtype of it. Go also falls back to simple names
 * (a receiver whose type is spelled `WebSettings`, or a `WebView`-typed
 * parameter chain through `settings`); a lookalike class with those names in
 * another package is a deliberate precision fix here. Resolution also sees
 * receivers Go's source inference cannot type (locals, class properties,
 * `getSettings()`, and implicit receivers such as
 * `settings.apply { javaScriptEnabled = true }`), which are the same API call.
 */
internal object SetJavaScriptEnabled : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SetJavaScriptEnabled"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SetJavaScriptEnabled)
        override val variableAssignmentCheckers = setOf(AssignmentChecker)
    }

    private const val MESSAGE = "Using setJavaScriptEnabled(true). Review for XSS vulnerabilities."
    private val WEB_SETTINGS = ClassId(FqName("android.webkit"), Name.identifier("WebSettings"))
    private val SETTER = Name.identifier("setJavaScriptEnabled")
    private val PROPERTY = Name.identifier("javaScriptEnabled")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name != SETTER || !isWebSettingsMember(callee)) return
        // Go reads the first positional (unlabeled) argument.
        val first = expression.argumentList.arguments.firstOrNull() ?: return
        if (first is FirNamedArgumentExpression) return
        if (!isBareTrueLiteral(first)) return
        report(expression.source, MESSAGE)
    }

    private object AssignmentChecker : FirVariableAssignmentChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirVariableAssignment) {
            val target = expression.unwrapLValue() ?: return
            val symbol = target.calleeReference.toResolvedCallableSymbol() ?: return
            if (symbol.name != PROPERTY || !isWebSettingsMember(symbol)) return
            if (!isBareTrueLiteral(expression.rValue)) return
            report(expression.source, MESSAGE)
        }
    }

    context(context: CheckerContext)
    private fun isWebSettingsMember(symbol: FirCallableSymbol<*>): Boolean {
        val owner = symbol.callableId?.classId ?: return false
        if (owner == WEB_SETTINGS) return true
        val session = context.session
        val ownerSymbol = session.symbolProvider.getClassLikeSymbolByClassId(owner) ?: return false
        return lookupSuperTypes(ownerSymbol, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.lookupTag.classId == WEB_SETTINGS }
    }

    private fun isBareTrueLiteral(expression: FirExpression): Boolean {
        val literal = expression as? FirLiteralExpression ?: return false
        if (literal.kind != ConstantValueKind.Boolean || literal.value != true) return false
        return literal.source?.isParenthesized() == false
    }

    // FIR drops parentheses, so read the syntax tree: Go only accepts a bare
    // `true`, not `(true)`.
    private fun KtSourceElement.isParenthesized(): Boolean = when (this) {
        is KtPsiSourceElement -> psi.parent is KtParenthesizedExpression
        is KtLightSourceElement -> treeStructure.getParent(lighterASTNode)?.tokenType == KtNodeTypes.PARENTHESIZED
        else -> false
    }
}
