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

// Flags `MessageDigest.getInstance("<weak algorithm>")` on the JDK's
// java.security.MessageDigest, where the first argument is a plain string
// literal naming MD2, MD4, MD5, SHA-1, or SHA1 (case-insensitive, surrounding
// whitespace ignored). Every getInstance overload counts; only the first
// argument is read.
//
// Mirrors the Go rule's argument evidence on top of FIR resolution:
// - the call must resolve to java.security.MessageDigest.getInstance;
// - the first argument is read from the call's syntax tree: it must be a
//   string template (optionally parenthesized) with no `$` entries, and the
//   algorithm is the raw text of its entries, so a literal containing an
//   escape sequence never matches;
// - surrounding whitespace is trimmed with Go's unicode.IsSpace set and the
//   text is uppercased code point by code point, as the Go rule does.
//
// Deliberate differences from Go, pinned by goldens. Go cannot resolve the
// receiver, so it accepts only the spellings `MessageDigest` and
// `java.security.MessageDigest` and guesses what a bare `MessageDigest` means
// from the file's imports and declarations; FIR reads the resolved call:
// - Precision: Go accepts a bare `MessageDigest` whenever the file imports or
//   mentions java.security.MessageDigest (a `java.security.*` star import
//   counts) and declares no MessageDigest itself, so it also reports when that
//   name resolves to another class: an explicit import of another class as or
//   named MessageDigest, or a same-package MessageDigest declared in another
//   file (which wins over a star import).
// - Recall (Go misses these true positives): an import alias, typealias,
//   parenthesized, or backticked spelling of java.security.MessageDigest, and
//   a statically imported getInstance (WeakMessageDigest); a bare
//   `MessageDigest` in a file that also declares an unrelated, non-shadowing
//   class named MessageDigest (WeakMessageDigestDeclaredName); an annotated or
//   labeled literal (`@A "MD5"`, `l@ "MD5"`), which Go reads as the
//   annotation or label node instead of the literal.
internal object WeakMessageDigest : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WeakMessageDigest"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WeakMessageDigest)
    }

    private const val MESSAGE =
        "MessageDigest.getInstance uses a weak digest algorithm. Use SHA-256, SHA-384, SHA-512, or SHA-3 for security-sensitive hashing."

    private val messageDigestClassId = ClassId(FqName("java.security"), Name.identifier("MessageDigest"))
    private val getInstanceId = CallableId(messageDigestClassId, Name.identifier("getInstance"))
    private val weakAlgorithms = setOf("MD2", "MD4", "MD5", "SHA-1", "SHA1")

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
    // folds `"${"MD5"}"` into a plain literal). Parentheses, annotations, and
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
