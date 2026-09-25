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
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `MessageDigest.getInstance("<weak algorithm>")` on the JDK's
// java.security.MessageDigest, where the first argument is a plain string
// literal naming MD2, MD4, MD5, SHA-1, or SHA1 (case-insensitive, surrounding
// whitespace ignored). Every getInstance overload counts; only the first
// argument is read.
//
// Mirrors the Go rule's evidence on top of FIR resolution:
// - the call must have an explicit receiver spelled `MessageDigest` or
//   `java.security.MessageDigest` (an import alias or typealias spelling, or a
//   static import of getInstance, is not flagged);
// - a bare `MessageDigest` receiver is not flagged when the file declares any
//   class, object, or typealias named MessageDigest;
// - the algorithm is the literal's raw source text between its quotes, so a
//   literal containing an escape sequence or a template never matches.
internal object WeakMessageDigest : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WeakMessageDigest"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WeakMessageDigest)
    }

    private const val MESSAGE =
        "MessageDigest.getInstance uses a weak digest algorithm. Use SHA-256, SHA-384, SHA-512, or SHA-3 for security-sensitive hashing."

    private val messageDigestClassId = ClassId(FqName("java.security"), Name.identifier("MessageDigest"))
    private const val SIMPLE_NAME = "MessageDigest"
    private const val QUALIFIED_NAME = "java.security.MessageDigest"
    private val weakAlgorithms = setOf("MD2", "MD4", "MD5", "SHA-1", "SHA1")

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId.classId != messageDigestClassId || callableId.callableName.asString() != "getInstance") return

        val receiver = expression.explicitReceiver as? FirResolvedQualifier ?: return
        if (receiver.classId != messageDigestClassId) return
        val receiverText = receiver.source?.text?.toString()?.trim() ?: return
        if (receiverText != SIMPLE_NAME && receiverText != QUALIFIED_NAME) return

        val first = expression.argumentList.arguments.firstOrNull() ?: return
        val literal = unwrap(first) as? FirLiteralExpression ?: return
        if (isTemplateEntry(expression, literal)) return
        val algorithm = rawStringLiteralContent(literal) ?: return
        if (algorithm.trim().uppercase() !in weakAlgorithms) return

        if (receiverText == SIMPLE_NAME) {
            val file = context.containingFileSymbol?.fir ?: return
            if (fileDeclaresMessageDigest(file)) return
        }

        report(expression.source, MESSAGE)
    }

    private fun unwrap(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    // K2 collapses a template whose only entry is a literal (`"${"MD5"}"`) into
    // that inner literal. Go sees an interpolated string there and skips it, so
    // a literal sitting directly inside `${ ... }` is not an argument literal.
    private fun isTemplateEntry(call: FirFunctionCall, literal: FirLiteralExpression): Boolean {
        val callSource = call.source ?: return true
        val literalSource = literal.source ?: return true
        val callText = callSource.text ?: return true
        var index = literalSource.startOffset - callSource.startOffset - 1
        while (index in callText.indices && (callText[index].isWhitespace() || callText[index] == '(')) {
            index--
        }
        return index in callText.indices && callText[index] == '{'
    }

    // The literal's source text between its delimiters, escapes left undecoded,
    // matching the Go rule's string_content reading. Parentheses around the
    // literal are stripped, as Go unwraps parenthesized arguments.
    private fun rawStringLiteralContent(literal: FirLiteralExpression): String? {
        if (literal.kind != ConstantValueKind.String) return null
        var text = literal.source?.text?.toString()?.trim() ?: return null
        while (text.length >= 2 && text.startsWith("(") && text.endsWith(")")) {
            text = text.substring(1, text.length - 1).trim()
        }
        return when {
            text.length >= 6 && text.startsWith("\"\"\"") && text.endsWith("\"\"\"") ->
                text.substring(3, text.length - 3)
            text.length >= 2 && text.startsWith("\"") && text.endsWith("\"") ->
                text.substring(1, text.length - 1)
            else -> null
        }
    }

    // Go skips a bare `MessageDigest` receiver when any class, interface,
    // object, or typealias named MessageDigest is declared anywhere in the file,
    // including nested and local declarations. Companion objects do not count.
    private fun fileDeclaresMessageDigest(file: FirFile): Boolean {
        var found = false
        file.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                val declares = when (element) {
                    is FirRegularClass -> !element.status.isCompanion && element.name.asString() == SIMPLE_NAME
                    is FirTypeAlias -> element.name.asString() == SIMPLE_NAME
                    else -> false
                }
                if (declares) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
