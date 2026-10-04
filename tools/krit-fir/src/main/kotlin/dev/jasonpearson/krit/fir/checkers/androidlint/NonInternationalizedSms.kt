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
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames

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
 * The call is a send method that belongs to a class whose simple name is
 * `SmsManager`, in any package, as Go matches the name: a member of
 * `android.telephony.SmsManager` (or the deprecated
 * `android.telephony.gsm.SmsManager`, whose methods take the same destination
 * first), of a third-party or project class, object, or companion named
 * `SmsManager`, or an extension on one. Go reads the receiver, not the callee,
 * so a function of that name declared on any type (`Any`, a type parameter)
 * counts too when its explicit receiver is an `SmsManager`, and so does a
 * property of that name invoked like a function. The destination is the
 * argument bound to the first parameter (a vararg's first element).
 *
 * Go cannot resolve the receiver, so it accepts a receiver spelled starting
 * with `SmsManager`, one whose simple name is `smsManager`, or a name declared
 * in an enclosing owner whose declaration mentions `SmsManager`. Deliberate
 * differences, pinned in golden data:
 * - Precision: a `smsManager`-named receiver of another type and a variable
 *   whose declaration mentions `SmsManager` without holding one reach no
 *   `SmsManager`, so they are not reported.
 * - Precision: Go tests the `+` prefix on the literal's source text without
 *   decoding escapes, so it reports a destination whose leading `+` is
 *   written as a unicode escape, although its value is an E.164 number; FIR
 *   reads the value and does not.
 * - Recall (Go misses these true positives): a fully qualified, nested, or
 *   aliased `SmsManager` (import or type alias), an annotated destination,
 *   `getSystemService(SmsManager::class.java)` and other receivers Go cannot
 *   type (`!!`, `it`, properties declared elsewhere), and implicit receivers
 *   such as `with(sms) { sendTextMessage(...) }`.
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

    private val SMS_MANAGER = Name.identifier("SmsManager")
    private val sendMethods = setOf(Name.identifier("sendTextMessage"), Name.identifier("sendMultipartTextMessage"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val sends = if (expression is FirImplicitInvokeCall) {
            isSmsSendProperty(expression.explicitReceiver, context.session)
        } else {
            callee.name in sendMethods && isSmsSend(callee, expression.explicitReceiver, context.session)
        }
        if (!sends) return
        val destination = callee.valueParameterSymbols.firstOrNull() ?: return
        val bound = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == destination }
            ?.key ?: return
        // A vararg destination is its first element, Go's first argument.
        val argument = if (bound is FirVarargArgumentsExpression) bound.arguments.firstOrNull() ?: return else bound
        val value = plainLiteralValue(argument) ?: return
        if (value.startsWith("+")) return
        report(expression.source, MESSAGE)
    }

    // A send method of a class named SmsManager (a member, or an extension on
    // it), or one called on an explicit SmsManager receiver: Go reads the
    // receiver, not the callee, so it reports an extension on Any or on a type
    // parameter called on an SmsManager too. Only class ids already in hand are
    // compared; none is looked up.
    private fun isSmsSend(callee: FirCallableSymbol<*>, explicitReceiver: FirExpression?, session: FirSession): Boolean =
        callee.callableId?.classId?.let(::isSmsManagerClass) == true ||
            isSmsManagerType(callee.resolvedReceiverType, session) ||
            isSmsManagerType(explicitReceiver?.resolvedType, session)

    // `sms.sendTextMessage(dest)` where sendTextMessage is a property of
    // function type (or of a type with an invoke operator): Go sees the same
    // call name and receiver as for a function.
    private fun isSmsSendProperty(invokeReceiver: FirExpression?, session: FirSession): Boolean {
        val access = invokeReceiver as? FirQualifiedAccessExpression ?: return false
        val property = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        return property.name in sendMethods && isSmsSend(property, access.explicitReceiver, session)
    }

    private fun isSmsManagerType(type: ConeKotlinType?, session: FirSession): Boolean {
        val classId = type?.fullyExpandedType(session)?.lowerBoundIfFlexible()?.classId ?: return false
        return isSmsManagerClass(classId)
    }

    // A class or object named SmsManager, in any package, as Go matches the
    // receiver by that name, or the companion object of one.
    private fun isSmsManagerClass(classId: ClassId): Boolean =
        classId.shortClassName == SMS_MANAGER ||
            (classId.shortClassName == SpecialNames.DEFAULT_NAME_FOR_COMPANION_OBJECT &&
                classId.outerClassId?.shortClassName == SMS_MANAGER)

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
