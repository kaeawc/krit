package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags an `AlarmManager.setRepeating` / `setInexactRepeating` call whose
 * interval (the third argument, `intervalMillis`) is a positive value below
 * 60 000 ms. The platform forces repeating alarms up to a minute, and short
 * alarms drain the battery.
 *
 * Like the Go rule, the interval must be a known number: an integer literal
 * (`5000L`, `30000`), not a parameter, a variable, a call, or an expression;
 * a value of 0 or below, and 60 000 or more, is not reported. The finding is
 * on the first line of the call expression (the receiver's first line for
 * `alarmManager.setRepeating(...)`), with Go's message.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go matches the call by name alone, whatever its receiver. This
 *   checker requires the resolved callee to be a member of
 *   `android.app.AlarmManager`, so a project scheduler (also one named
 *   `AlarmManager`), a wrapper, a local or anonymous class, or an extension
 *   function named `setRepeating` is not reported: none of them sets a
 *   platform alarm's repeat interval (ShortAlarmLookalike,
 *   ShortAlarmThirdParty).
 * - Recall: FIR reads the literal's value, so an interval Go cannot parse is
 *   reported: a literal with underscores (`5_000L`), a hex or binary literal,
 *   a parenthesized literal, and a reference to a Kotlin `const val` whose
 *   value is such a literal (ShortAlarmRecall).
 */
internal object ShortAlarm : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ShortAlarm"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ShortAlarm)
    }

    private const val MESSAGE =
        "Short alarm interval. Consider using a minimum of 60 seconds for repeating alarms."
    private const val MINIMUM_INTERVAL = 60_000L
    private const val INTERVAL_INDEX = 2
    private val alarmManager = ClassId(FqName("android.app"), Name.identifier("AlarmManager"))
    private val repeatingSetters = setOf(Name.identifier("setRepeating"), Name.identifier("setInexactRepeating"))
    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)
    private val integralKinds = setOf(
        ConstantValueKind.Int,
        ConstantValueKind.Long,
        ConstantValueKind.Short,
        ConstantValueKind.Byte,
        ConstantValueKind.IntegerLiteral,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name !in repeatingSetters) return
        if (callee.receiverParameterSymbol != null) return
        val owner = callee.getContainingClassSymbol() ?: return
        if (!isAlarmManager(owner)) return
        val intervalParameter = callee.valueParameterSymbols.getOrNull(INTERVAL_INDEX) ?: return
        val interval = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == intervalParameter }
            ?.key
            ?: return
        val millis = integralValue(interval, HashSet()) ?: return
        if (millis <= 0 || millis >= MINIMUM_INTERVAL) return
        val source = expression.source ?: return
        report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
    }

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun isAlarmManager(symbol: FirClassLikeSymbol<*>): Boolean =
        symbol.classId == alarmManager ||
            lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == alarmManager }

    // The value of an integer literal (FIR drops parentheses around it and
    // folds underscores, hex, and binary spellings), or of a Kotlin const val
    // whose initializer is one; null for anything else.
    private fun integralValue(expression: FirExpression, seen: MutableSet<FirPropertySymbol>): Long? = when (expression) {
        is FirLiteralExpression ->
            if (expression.kind in integralKinds) (expression.value as? Number)?.toLong() else null
        is FirPropertyAccessExpression -> {
            val property = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol
            if (property == null || !property.isConst || !seen.add(property)) {
                null
            } else {
                property.resolvedInitializer?.let { integralValue(it, seen) }
            }
        }
        else -> null
    }

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call. K2 gives a dot call the whole qualified expression as its source;
    // a safe call keeps the selector call expression, so step up to its parent.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = source.treeStructure.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = lightChildren(source, parent).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS
        }
        return parent.takeIf { parts.size > 1 && parts.last() == node }
    }
}
