package dev.jasonpearson.krit.fir.checkers.security

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.trimGoSpace
import dev.jasonpearson.krit.fir.support.upperCodePoints
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirNamedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.util.getChildren

// Flags `MessageDigest.getInstance("<weak algorithm>")` on the JDK's
// java.security.MessageDigest, where the first argument is a plain string
// literal naming MD2, MD4, MD5, SHA, SHA-1, or SHA1 (case-insensitive, surrounding
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
    private val weakAlgorithms = setOf("MD2", "MD4", "MD5", "SHA-1", "SHA1", "SHA")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != getInstanceId) return

        if (expression.argumentList.arguments.isEmpty()) return
        val algorithm = firstArgumentLiteralContent(expression) ?: return
        if (upperCodePoints(trimGoSpace(algorithm)) !in weakAlgorithms) return

        if (isNonSecurityUse(expression)) return

        report(expression.source, MESSAGE)
    }

    private val securityHints = listOf("password", "passwd", "token", "signature", "secret", "credential", "hmac", "salt", "auth")
    private val benignHints = listOf("checksum", "hash", "gravatar", "avatar", "etag", "cachekey", "fingerprint", "diagnostic")

    // Go's weakMessageDigestIsNonSecurityUse: names around the call say the digest is a checksum, cache key or
    // id. The enclosing function, nearest class, the property the call initializes, and the names that receive
    // the digest() output are the evidence.
    context(context: CheckerContext)
    private fun isNonSecurityUse(call: FirFunctionCall): Boolean {
        val elements = context.containingElements
        val function = elements.filterIsInstance<FirNamedFunction>().lastOrNull()
        val cls = elements.filterIsInstance<FirRegularClass>().lastOrNull { it.classKind != ClassKind.OBJECT }
        val property = elements.lastOrNull { it is FirNamedFunction || it is FirProperty } as? FirProperty
        val names = listOfNotNull(function?.name?.asString(), cls?.name?.asString(), property?.name?.asString())

        val flow = if (function != null) resultNames(function, call, property) else emptyList()
        if (flow.isNotEmpty()) {
            val flowJoined = flow.joinToString(" ").lowercase()
            if (!hasSecurityHint(flowJoined, flow) && benignHints.any { it in flowJoined }) return true
        }

        val joined = names.joinToString(" ").lowercase()
        if (securityHints.any { it in joined }) return false
        val words = names.flatMap(::identifierWords)
        if ("sign" in words) return false
        if (benignHints.any { it in joined }) return true
        return "id" in words
    }

    private fun hasSecurityHint(joined: String, names: List<String>): Boolean =
        securityHints.any { it in joined } || names.any { "sign" in identifierWords(it) }

    // Go's splitIdentifierWords: lowercase words split at `_`, `-`, space, backtick and before each uppercase letter.
    private fun identifierWords(name: String): List<String> {
        val words = ArrayList<String>()
        val cur = StringBuilder()
        fun flush() {
            if (cur.isNotEmpty()) words += cur.toString().lowercase()
            cur.setLength(0)
        }
        for (c in name) {
            when {
                c == '_' || c == '-' || c == ' ' || c == '`' -> flush()
                c in 'A'..'Z' -> { flush(); cur.append(c) }
                else -> cur.append(c)
            }
        }
        flush()
        return words
    }

    // The names that receive the output of digest() on the getInstance result [call]: a property it initializes,
    // an assignment target, or a named argument.
    private fun resultNames(function: FirNamedFunction, call: FirFunctionCall, digestVar: FirProperty?): List<String> {
        val names = ArrayList<String>()
        fun isDigestOnTarget(e: FirElement?): Boolean {
            val fc = e as? FirFunctionCall ?: return false
            if (fc.calleeReference.name.asString() != "digest") return false
            val receiver = fc.explicitReceiver
            if (receiver === call) return true
            val symbol = (receiver as? FirPropertyAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
            return digestVar != null && symbol == digestVar.symbol
        }
        function.accept(object : FirVisitorVoid() {
            private val owners = ArrayDeque<String>()

            override fun visitElement(element: FirElement) {
                if (isDigestOnTarget(element)) owners.lastOrNull()?.let(names::add)
                if (element is FirVariableAssignment && isDigestOnTarget(element.rValue)) {
                    (element.lValue as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()?.name?.asString()?.let(names::add)
                }
                if (element is FirNamedArgumentExpression && isDigestOnTarget(element.expression)) names += element.name.asString()
                if (element is FirProperty) {
                    owners.addLast(element.name.asString())
                    element.acceptChildren(this)
                    owners.removeLast()
                    return
                }
                element.acceptChildren(this)
            }
        })
        return names
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
}
