package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

/**
 * Flags `javax.crypto.Cipher.getInstance("RSA/<mode>/NoPadding")`: textbook RSA
 * without padding. Mirrors the Go RsaNoPadding rule on Kotlin code:
 *  - the call resolves to `javax.crypto.Cipher.getInstance` through an explicit
 *    receiver that resolves to `javax.crypto.Cipher` and is spelled `Cipher` or
 *    `javax.crypto.Cipher`, as Go's receiver-text check requires (an aliased
 *    import or a statically imported `getInstance` does not fire, as in Go);
 *  - the first argument is a string literal without interpolation whose trimmed,
 *    upper-cased source content (escape sequences undecoded, as Go reads it)
 *    splits on `/` into exactly `RSA`, a non-empty mode, and `NOPADDING`.
 * The finding is reported on the call expression, which starts on the line of
 * the receiver, as Go reports on the start of its call_expression.
 *
 * Deliberate differences from Go, pinned by golden tests. Go cannot resolve a
 * bare `Cipher`, so it guesses from the file's import headers and declarations;
 * FIR reads the resolved receiver instead:
 *  - Go false negatives FIR reports: a class or object named `Cipher` nested
 *    in another class of the file (RsaNoPaddingFileDeclaresCipher). A nested
 *    `fun interface Cipher` or backticked `Cipher` also never suppresses, and
 *    Go reports those too because its guard misses them
 *    (RsaNoPaddingFunInterface, RsaNoPaddingBacktickedDeclaration);
 *    a comment or KDoc after the Cipher import, which tree-sitter folds into
 *    the import header text Go compares (RsaNoPaddingImportComment,
 *    RsaNoPaddingImportKDoc); an annotated or labeled literal argument
 *    (RsaNoPaddingAnnotatedArgument).
 *  - Go false positives FIR skips: a local val, parameter, or companion object
 *    named `Cipher` that shadows the import (RsaNoPaddingShadowed,
 *    RsaNoPaddingCompanionShadow); an explicitly imported other `Cipher` under
 *    a `javax.crypto.*` import (RsaNoPaddingImportedOtherCipher), and likewise
 *    a `Cipher` class in another file of the same package; a raw string with
 *    an extra closing quote, whose value ends in `"` (RsaNoPaddingShadowed).
 */
internal object RsaNoPadding : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "RsaNoPadding"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(RsaNoPadding)
    }

    private val cipherClassId = ClassId(FqName("javax.crypto"), Name.identifier("Cipher"))
    private val getInstanceId = CallableId(cipherClassId, Name.identifier("getInstance"))

    private const val MESSAGE =
        "RSA cipher uses NoPadding. Use OAEPWithSHA-256AndMGF1Padding or at minimum PKCS1Padding instead of textbook RSA."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != getInstanceId) return

        // The resolved classId already rules out a shadowing `Cipher`, so Go's
        // import and same-file declaration guesses are not needed.
        val receiver = expression.explicitReceiver as? FirResolvedQualifier ?: return
        if (receiver.classId != cipherClassId) return
        val receiverText = receiver.source?.text?.toString()?.trim() ?: return
        if (receiverText != "Cipher" && receiverText != "javax.crypto.Cipher") return

        val algorithm = firstStringLiteral(expression) ?: return
        if (!isRsaNoPadding(algorithm)) return

        report(expression.source, MESSAGE)
    }

    // Returns the literal's content as Go reads it: the source text between the
    // quotes, with escape sequences left undecoded (tree-sitter's
    // string_content). A raw string has no escapes, so its content equals the
    // runtime value. Interpolated strings are not FirLiteralExpressions.
    private fun firstStringLiteral(call: FirFunctionCall): String? {
        val first = call.argumentList.arguments.firstOrNull() ?: return null
        val literal = unwrap(first) as? FirLiteralExpression ?: return null
        if (literal.value !is String) return null
        val text = literal.source?.text?.toString()?.trim() ?: return null
        return when {
            text.length >= 6 && text.startsWith(RAW_QUOTE) && text.endsWith(RAW_QUOTE) ->
                text.substring(RAW_QUOTE.length, text.length - RAW_QUOTE.length)
            text.length >= 2 && text.startsWith(QUOTE) && text.endsWith(QUOTE) ->
                text.substring(QUOTE.length, text.length - QUOTE.length)
            else -> null
        }
    }

    private const val QUOTE = "\""
    private const val RAW_QUOTE = "\"\"\""

    private fun unwrap(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    private fun isRsaNoPadding(algorithm: String): Boolean {
        val parts = algorithm.trim().uppercase().split("/")
        return parts.size == 3 && parts[0] == "RSA" && parts[1].isNotEmpty() && parts[2] == "NOPADDING"
    }
}
