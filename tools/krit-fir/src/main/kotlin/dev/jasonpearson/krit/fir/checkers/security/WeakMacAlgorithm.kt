package dev.jasonpearson.krit.fir.checkers.security

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.util.getChildren

// Flags `Mac.getInstance("<weak HMAC>")` on the JDK's javax.crypto.Mac, where
// the first argument is a plain string literal naming HmacMD2, HmacMD5,
// HmacSHA0, or HmacSHA1 (case-insensitive, surrounding whitespace ignored).
// Every getInstance overload counts; only the first argument is read.
//
// Mirrors the Go rule's argument evidence on top of FIR resolution:
// - the call must resolve to javax.crypto.Mac.getInstance;
// - the first argument is read from the call's syntax tree: it must be a
//   string template (optionally parenthesized) with no `$` entries, and the
//   algorithm is the raw text of its entries, so a literal containing an
//   escape sequence never matches;
// - surrounding whitespace is trimmed with Go's unicode.IsSpace set and the
//   text is uppercased code point by code point, as the Go rule does.
//
// Deliberate differences from Go, pinned by goldens. Go cannot resolve the
// receiver, so it accepts only the spellings `Mac` and `javax.crypto.Mac` and
// guesses what a bare `Mac` means from the file's imports and declarations;
// FIR reads the resolved call instead:
// - Recall (Go misses these true positives): an import alias, typealias, or
//   backticked spelling of javax.crypto.Mac, and a statically imported
//   getInstance (WeakMacAlgorithmSpellings); a bare `Mac` in a file that also
//   declares an unrelated, non-shadowing top-level, nested, or local class or
//   nested object named Mac (WeakMacAlgorithmDeclaredName); a top-level
//   `typealias Mac = javax.crypto.Mac`, which Go counts as a lookalike
//   declaration (WeakMacAlgorithmTypealiasNamedMac); a parenthesized receiver
//   `(Mac)`, an annotated or labeled literal (`@A "HmacMD5"`,
//   `l@ "HmacMD5"`), or a parenthesized literal preceded by a comment, which
//   Go reads as the argument node instead of the literal (WeakMacAlgorithm).
// - Precision (Go reports these, but the call is not javax.crypto.Mac's):
//   a local val, parameter, member property, or companion object named Mac
//   that shadows the import (WeakMacAlgorithmShadowed), and a top-level
//   property named Mac (WeakMacAlgorithmShadowedTopLevel); an explicit import
//   of another class as or named Mac over a `javax.crypto.*` star import
//   (WeakMacAlgorithmAliasedOther); and, in WeakMacAlgorithmTest, a
//   same-package Mac declared in another file over a star import, an explicit
//   import of another Mac where Go's javax.crypto.Mac mention check is met by
//   a comment, a `javax.crypto.MacSpi` import, or `javax.crypto.Mac as JMac`,
//   and a nested Mac inherited from a supertype declared in another file.
internal object WeakMacAlgorithm : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WeakMacAlgorithm"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WeakMacAlgorithm)
    }

    private const val MESSAGE =
        "Mac.getInstance uses an HMAC algorithm backed by a weak digest. Use HmacSHA256, HmacSHA384, HmacSHA512, or SHA-3-based alternatives."

    private val macClassId = ClassId(FqName("javax.crypto"), Name.identifier("Mac"))
    private val getInstanceId = CallableId(macClassId, Name.identifier("getInstance"))
    private val weakAlgorithms = setOf("HMACMD2", "HMACMD5", "HMACSHA0", "HMACSHA1")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != getInstanceId) return

        if (expression.argumentList.arguments.isEmpty()) return
        val algorithm = firstArgumentLiteralContent(expression) ?: return
        if (upperCodePoints(trimGoSpace(algorithm)) !in weakAlgorithms) return

        report(expression.source, MESSAGE)
    }

    // Syntax wrappers that leave the argument's value unchanged, and the
    // non-expression parts inside them.
    private val expressionWrappers = setOf(
        KtNodeTypes.PARENTHESIZED,
        KtNodeTypes.ANNOTATED_EXPRESSION,
        KtNodeTypes.LABELED_EXPRESSION,
    )
    private val wrapperParts = setOf(
        KtTokens.LPAR,
        KtTokens.RPAR,
        KtNodeTypes.ANNOTATION_ENTRY,
        KtNodeTypes.ANNOTATION,
        KtNodeTypes.LABEL_QUALIFIER,
    )

    // The raw text of the first argument when it is a string template without
    // `$` entries, read from the call's syntax tree rather than from FIR (K2
    // folds `"${"HmacMD5"}"` into a plain literal). Parentheses, annotations, and
    // labels around the literal are unwrapped, and escape entries keep their
    // source text, as the Go rule reads them.
    private fun firstArgumentLiteralContent(call: FirFunctionCall): String? {
        val source = call.source ?: return null
        val tree = source.treeStructure
        val root = source.lighterASTNode
        val callExpression = when (root.tokenType) {
            KtNodeTypes.CALL_EXPRESSION -> root
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION ->
                significantChildren(root, tree).lastOrNull()?.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
            else -> null
        } ?: return null
        val argumentList = significantChildren(callExpression, tree)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val argument = significantChildren(argumentList, tree)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
        // A named (`name = x`) or spread (`*x`) argument has more than one part.
        var value = significantChildren(argument, tree).singleOrNull() ?: return null
        while (value.tokenType in expressionWrappers) {
            value = significantChildren(value, tree)
                .filter { it.tokenType !in wrapperParts }
                .singleOrNull() ?: return null
        }
        if (value.tokenType != KtNodeTypes.STRING_TEMPLATE) return null
        val content = StringBuilder()
        for (part in value.getChildren(tree)) {
            when (part.tokenType) {
                KtTokens.OPEN_QUOTE, KtTokens.CLOSING_QUOTE -> Unit
                KtNodeTypes.LITERAL_STRING_TEMPLATE_ENTRY, KtNodeTypes.ESCAPE_STRING_TEMPLATE_ENTRY ->
                    content.append(tree.toString(part))
                else -> return null
            }
        }
        return content.toString()
    }

    private fun significantChildren(
        node: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): List<LighterASTNode> =
        node.getChildren(tree).filter { it.tokenType != KtTokens.WHITE_SPACE && it.tokenType !in KtTokens.COMMENTS }

    // Go's strings.TrimSpace trims runes where unicode.IsSpace holds: \t \n \v
    // \f \r, space, U+0085 and U+00A0, plus the Unicode White_Space characters
    // above Latin-1. Kotlin's trim() differs: it trims U+001C..U+001F and
    // keeps U+0085.
    private fun trimGoSpace(text: String): String = text.trim(::isGoSpace)

    private fun isGoSpace(c: Char): Boolean = when (c) {
        '\t', '\n', '\u000B', '\u000C', '\r', ' ', '\u0085', ' ',
        ' ', ' ', ' ', ' ', ' ', '　' -> true
        else -> c in ' '..' '
    }

    // Go's strings.ToUpper maps each code point to its simple uppercase form,
    // while String.uppercase() applies full mappings (one char to several).
    private fun upperCodePoints(text: String): String {
        val out = StringBuilder(text.length)
        var i = 0
        while (i < text.length) {
            val cp = text.codePointAt(i)
            out.appendCodePoint(Character.toUpperCase(cp))
            i += Character.charCount(cp)
        }
        return out.toString()
    }
}
