package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirTypeAlias
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

/**
 * Flags `javax.crypto.Cipher.getInstance("RSA/<mode>/NoPadding")`: textbook RSA
 * without padding. Mirrors the Go RsaNoPadding rule on Kotlin code:
 *  - the call resolves to `javax.crypto.Cipher.getInstance` through an explicit
 *    receiver spelled `Cipher` or `javax.crypto.Cipher`;
 *  - a bare `Cipher` receiver additionally needs an `import javax.crypto.Cipher`
 *    or `import javax.crypto.*`, and no class, object, or type alias named
 *    `Cipher` declared anywhere in the file (Go's same-file lookalike guard);
 *  - the first argument is a string literal without interpolation whose trimmed,
 *    upper-cased source content (escape sequences undecoded, as Go reads it)
 *    splits on `/` into exactly `RSA`, a non-empty mode, and `NOPADDING`.
 * The finding is reported on the call expression, which starts on the line of
 * the receiver, as Go reports on the start of its call_expression.
 */
internal object RsaNoPadding : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "RsaNoPadding"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(RsaNoPadding)
    }

    private val cipherClassId = ClassId(FqName("javax.crypto"), Name.identifier("Cipher"))
    private val getInstanceId = CallableId(cipherClassId, Name.identifier("getInstance"))
    private val cipherFqName = cipherClassId.asSingleFqName()
    private val cryptoPackage = cipherClassId.packageFqName

    private const val MESSAGE =
        "RSA cipher uses NoPadding. Use OAEPWithSHA-256AndMGF1Padding or at minimum PKCS1Padding instead of textbook RSA."

    // Reading the containing FirFile (imports and declarations) needs the
    // symbol's `fir`, as the oracle checkers do for the file path.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != getInstanceId) return

        val receiver = expression.explicitReceiver as? FirResolvedQualifier ?: return
        if (receiver.classId != cipherClassId) return
        val receiverText = receiver.source?.text?.toString()?.trim() ?: return

        val algorithm = firstStringLiteral(expression) ?: return
        if (!isRsaNoPadding(algorithm)) return

        when (receiverText) {
            "javax.crypto.Cipher" -> Unit
            "Cipher" -> {
                val file = context.containingFileSymbol?.fir ?: return
                if (!importsJavaxCipher(file) || declaresCipherType(file)) return
            }
            else -> return
        }

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

    // Go accepts only `import javax.crypto.Cipher` or `import javax.crypto.*`;
    // an aliased import does not count.
    private fun importsJavaxCipher(file: FirFile): Boolean =
        file.imports.any { import ->
            if (import.aliasName != null) return@any false
            val fqName = import.importedFqName ?: return@any false
            if (import.isAllUnder) fqName == cryptoPackage else fqName == cipherFqName
        }

    // Go's same-file guard: any class, interface, enum, object, or type alias
    // named `Cipher` in the file, at any depth. Companion objects are not
    // counted, matching tree-sitter's separate `companion_object` node.
    private fun declaresCipherType(file: FirFile): Boolean {
        var found = false
        file.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                when (element) {
                    is FirRegularClass ->
                        if (!element.isCompanion && element.name.asString() == "Cipher") {
                            found = true
                            return
                        }
                    is FirTypeAlias ->
                        if (element.name.asString() == "Cipher") {
                            found = true
                            return
                        }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
