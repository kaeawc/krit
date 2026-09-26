package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags an SMS sent to a destination written as a plain string literal that
 * does not start with `+`: `SmsManager.sendTextMessage` or
 * `sendMultipartTextMessage` whose first argument (`destinationAddress`) is a
 * string literal without `$` templates whose value does not begin with the
 * E.164 `+`. The finding sits on the first line of the call expression,
 * receiver included, where Go reports it.
 *
 * Like the Go rule:
 * - only a literal counts; a variable, constant, concatenation, or template is
 *   dynamic and never reported;
 * - parentheses around the literal are looked through;
 * - the literal counts by its runtime value, so leading whitespace or a
 *   newline before the `+` is reported, and a raw string holds no escapes.
 *
 * The call must resolve to a member of `android.telephony.SmsManager` (or the
 * deprecated `android.telephony.gsm.SmsManager`, whose methods take the same
 * destination first), or to a project extension of that name on it, which Go
 * reports through its receiver as well; the destination is the argument bound
 * to the first parameter. Go cannot resolve the receiver, so it accepts a
 * receiver spelled starting with `SmsManager`, one whose simple name is
 * `smsManager`, or a name declared in an enclosing owner whose declaration
 * mentions `SmsManager`. Deliberate differences, pinned in golden data:
 * - Precision: a `smsManager`-named receiver of another type, a project
 *   class named `SmsManager`, and a variable whose declaration mentions
 *   `SmsManager` without holding one do not send an SMS through the platform
 *   API, so they are not reported.
 * - Precision: Go tests the `+` prefix on the literal's source text without
 *   decoding escapes, so it reports a destination whose leading `+` is
 *   written as a unicode escape, although its value is an E.164 number; FIR
 *   reads the value and does not.
 * - Recall (Go misses these true positives): a fully qualified or aliased
 *   `SmsManager`, `getSystemService(SmsManager::class.java)` and other
 *   receivers Go cannot type (`!!`, properties declared elsewhere), and
 *   implicit receivers such as `with(sms) { sendTextMessage(...) }`.
 * - Named arguments (only possible on a project extension): Go reads the
 *   first unlabeled argument, FIR the argument bound to the first parameter,
 *   so a named literal destination is reported and a literal passed
 *   positionally after a named destination is not.
 */
internal object NonInternationalizedSms : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "NonInternationalizedSms"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(NonInternationalizedSms)
    }

    private const val MESSAGE =
        "SMS destination should use international E.164 format starting with '+' to deliver correctly when roaming."

    private val smsManagerClassIds = setOf(
        ClassId(FqName("android.telephony"), Name.identifier("SmsManager")),
        ClassId(FqName("android.telephony.gsm"), Name.identifier("SmsManager")),
    )
    private val sendMethods = setOf(Name.identifier("sendTextMessage"), Name.identifier("sendMultipartTextMessage"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name !in sendMethods || !isSmsManagerFunction(callee, context.session)) return
        val destination = callee.valueParameterSymbols.firstOrNull() ?: return
        val argument = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == destination }
            ?.key ?: return
        val value = plainLiteralValue(argument) ?: return
        if (value.startsWith("+")) return
        report(expression.source, MESSAGE)
    }

    // A member of SmsManager, or an extension on it (Go reads the receiver,
    // not the callee, so it reports a project extension called on an
    // SmsManager too). SmsManager is final, so members are declared on it.
    private fun isSmsManagerFunction(callee: FirFunctionSymbol<*>, session: FirSession): Boolean {
        val receiverType = callee.resolvedReceiverType
            ?: return callee.callableId.classId in smsManagerClassIds
        return receiverType.fullyExpandedType(session).lowerBoundIfFlexible().classId in smsManagerClassIds
    }

    // The value of [argument] when it is written as a string literal with no
    // `$` entries (optionally parenthesized). The syntax decides whether it is
    // a literal, because K2 folds a constant template such as `"${"555"}"`
    // into a plain literal, which Go treats as dynamic.
    private fun plainLiteralValue(argument: FirExpression): String? {
        val literal = argument as? FirLiteralExpression ?: return null
        val value = literal.value as? String ?: return null
        val source = argument.source ?: return null
        val template = unwrapParentheses(source, source.lighterASTNode) ?: return null
        if (template.tokenType != KtNodeTypes.STRING_TEMPLATE) return null
        val hasTemplateEntry = lightChildren(source, template).any {
            it.tokenType == KtNodeTypes.SHORT_STRING_TEMPLATE_ENTRY ||
                it.tokenType == KtNodeTypes.LONG_STRING_TEMPLATE_ENTRY
        }
        return value.takeUnless { hasTemplateEntry }
    }

    private fun unwrapParentheses(source: KtSourceElement, node: LighterASTNode): LighterASTNode? {
        var current = node
        while (current.tokenType == KtNodeTypes.PARENTHESIZED) {
            current = lightChildren(source, current).singleOrNull {
                it.tokenType != KtTokens.LPAR &&
                    it.tokenType != KtTokens.RPAR &&
                    it.tokenType != KtTokens.WHITE_SPACE &&
                    it.tokenType !in KtTokens.COMMENTS
            } ?: return null
        }
        return current
    }
}
