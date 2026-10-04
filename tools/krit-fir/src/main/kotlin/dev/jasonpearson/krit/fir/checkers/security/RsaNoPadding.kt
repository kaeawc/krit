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
 *  - the call resolves to `javax.crypto.Cipher.getInstance`;
 *  - the first argument is a string literal without interpolation whose trimmed,
 *    upper-cased source content (escape sequences undecoded, as Go reads it)
 *    splits on `/` into exactly `RSA`, a non-empty mode, and `NOPADDING`.
 * The finding is reported on the call expression, which starts on the line of
 * the receiver, as Go reports on the start of its call_expression.
 *
 * Deliberate differences from Go, pinned by golden tests. Go cannot resolve a
 * bare `Cipher`, so it reads the file's import paths and the declarations in
 * scope at the call; FIR reads the resolved receiver instead:
 *  - Go false negatives FIR reports: any spelling of the receiver other than
 *    `Cipher` or `javax.crypto.Cipher`: an import alias, a typealias, a
 *    parenthesized or backticked receiver, or a statically imported
 *    `getInstance`, plain or aliased (RsaNoPaddingSpellings), and a qualified
 *    receiver split across lines (RsaNoPaddingNoImport); an annotated or
 *    labeled literal argument (RsaNoPaddingAnnotatedArgument).
 *  - Go false positives FIR skips: a `Cipher` class in another file of the
 *    same package under a `javax.crypto.*` import; a raw string with an extra
 *    closing quote, whose value ends in `"` (RsaNoPaddingShadowed).
 * Go and FIR agree, also pinned by golden tests, on a `Cipher` nested in a
 * class that does not enclose the call, which does not shadow the import
 * (RsaNoPaddingFileDeclaresCipher, RsaNoPaddingFunInterface,
 * RsaNoPaddingBacktickedDeclaration); on a comment or KDoc that tree-sitter
 * folds into the import header (RsaNoPaddingImportComment,
 * RsaNoPaddingImportKDoc); and on a local val, parameter, or companion object
 * of the calling class named `Cipher`, or another type explicitly imported as
 * `Cipher`, which shadow it (RsaNoPaddingShadowed, RsaNoPaddingCompanionShadow,
 * RsaNoPaddingImportedOtherCipher).
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

        // The resolved callable already rules out a shadowing `Cipher`, so Go's
        // receiver spelling, import, and same-file declaration guesses are not
        // needed.
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
