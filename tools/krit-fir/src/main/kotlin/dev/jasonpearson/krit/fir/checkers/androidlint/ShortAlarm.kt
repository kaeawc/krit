package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
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
 * Go matches the call by name alone, whatever its receiver. The message is
 * about the alarm the call sets, so this checker reports a call whose
 * resolved callee sets a platform repeating alarm from its third argument: a
 * member of `android.app.AlarmManager` (or a subclass); an extension on
 * AlarmManager whose third parameter is a Long; a function whose body it
 * cannot read (Java, a library, an interface member), as Go does; or a Kotlin
 * wrapper whose body passes its third parameter into the interval of one of
 * these (ShortAlarmForwarding).
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: a project scheduler (also one named `AlarmManager`), a local
 *   or anonymous class, a top-level function, or an extension whose body
 *   never passes its third parameter to an AlarmManager interval (an empty
 *   lookalike, a retry count, a wrapper that sets a fixed hourly alarm) sets
 *   no alarm interval from the literal, so it is not reported
 *   (ShortAlarmLookalike, ShortAlarmThirdParty, ShortAlarmForwarding).
 * - Recall: FIR reads the literal's value, so an interval Go cannot parse is
 *   reported: a literal with underscores (`5_000L`), a hex or binary literal,
 *   a parenthesized literal, and a reference to a Kotlin `const val` whose
 *   value is such a literal (ShortAlarmRecall).
 * - Tree-sitter joins a statement that starts with `(` to the previous line's
 *   call, so Go reports the earlier line and misses the later one; FIR reads
 *   the statements as Kotlin does (ShortAlarmDivergence).
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
    private val alarmManagerType = alarmManager.constructClassLikeType(emptyArray(), isMarkedNullable = true)
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
        val interval = intervalArgument(expression, callee) ?: return
        val millis = integralValue(interval, HashSet()) ?: return
        if (millis <= 0 || millis >= MINIMUM_INTERVAL) return
        if (!setsAlarmInterval(callee, HashSet())) return
        val source = expression.source ?: return
        report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
    }

    // The argument bound to the callee's third value parameter.
    private fun intervalArgument(call: FirFunctionCall, callee: FirNamedFunctionSymbol): FirExpression? {
        val parameter = callee.valueParameterSymbols.getOrNull(INTERVAL_INDEX) ?: return null
        return call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == parameter }?.key
    }

    // Whether a call to [function] (named like a repeating setter) sets a
    // platform repeating alarm whose interval comes from its third parameter:
    // - a member of AlarmManager or a subclass;
    // - an extension on AlarmManager (a subtype, or nullable) whose third
    //   parameter is a Long, as a project helper around the member is;
    // - a function whose body FIR cannot read (Java, a library, an abstract
    //   or interface member): Go reports it and nothing shows the message is
    //   false, so it is reported too;
    // - a Kotlin function whose body passes its third parameter into the
    //   interval of a call that is one of these (a wrapper, however deep).
    // A function whose body never passes the parameter on (an empty
    // lookalike, a retry count) sets no alarm interval from it.
    context(context: CheckerContext)
    private fun setsAlarmInterval(function: FirNamedFunctionSymbol, seen: MutableSet<FirNamedFunctionSymbol>): Boolean {
        val original = function.unwrapFakeOverrides()
        val parameter = original.valueParameterSymbols.getOrNull(INTERVAL_INDEX) ?: return false
        val receiverType = original.resolvedReceiverType
        if (receiverType == null) {
            val owner = original.getContainingClassSymbol()
            if (owner != null && isAlarmManager(owner)) return true
        } else if (isLong(parameter.resolvedReturnType) && receiverType.isSubtypeOf(alarmManagerType, context.session)) {
            return true
        }
        if (!seen.add(original)) return false
        val body = bodyOf(original) ?: return true
        return passesToInterval(body, parameter, seen)
    }

    @OptIn(SymbolInternals::class)
    private fun bodyOf(function: FirNamedFunctionSymbol): FirBlock? = function.fir.body

    // Whether [body] holds a repeating-setter call (see [setsAlarmInterval])
    // whose interval argument reads [parameter], directly or in an expression
    // such as `seconds * 1000L`. The parameter is matched by symbol, so a
    // shadowing name in a lambda or local function does not count.
    context(context: CheckerContext)
    private fun passesToInterval(
        body: FirElement,
        parameter: FirValueParameterSymbol,
        seen: MutableSet<FirNamedFunctionSymbol>,
    ): Boolean {
        var found = false
        body.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && forwardsInterval(element, parameter, seen)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    context(context: CheckerContext)
    private fun forwardsInterval(
        call: FirFunctionCall,
        parameter: FirValueParameterSymbol,
        seen: MutableSet<FirNamedFunctionSymbol>,
    ): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name !in repeatingSetters) return false
        val interval = intervalArgument(call, callee) ?: return false
        return reads(interval, parameter) && setsAlarmInterval(callee, seen)
    }

    private fun reads(expression: FirExpression, parameter: FirValueParameterSymbol): Boolean {
        var found = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirPropertyAccessExpression && element.calleeReference.toResolvedCallableSymbol() == parameter) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    context(context: CheckerContext)
    private fun isLong(type: ConeKotlinType): Boolean =
        type.fullyExpandedType().lowerBoundIfFlexible().classId == StandardClassIds.Long

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
