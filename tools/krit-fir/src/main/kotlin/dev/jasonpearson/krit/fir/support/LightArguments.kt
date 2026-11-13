package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

// Light-tree readers for the first argument of a call, the way Go reads it from the syntax tree.
// FIR is not used because K2 folds `"${"MD5"}"` into a plain literal.

private val callParentTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

// Syntax wrappers that leave an argument's value unchanged, and the non-expression parts inside them.
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

/**
 * The first argument's expression node of the call that [source] spans, with parentheses, annotations, and
 * labels unwrapped. Null when the call has no arguments, or the first one is named or spread.
 */
fun firstArgumentNode(source: KtSourceElement): LighterASTNode? {
    val root = source.lighterASTNode
    val call = when (root.tokenType) {
        KtNodeTypes.CALL_EXPRESSION -> root
        in callParentTypes ->
            significantChildren(source, root).lastOrNull()?.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
        else -> null
    } ?: return null
    val arguments = significantChildren(source, call)
        .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
    val argument = significantChildren(source, arguments)
        .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
    // A named (`name = x`) or spread (`*x`) argument has more than one part.
    var value = significantChildren(source, argument).singleOrNull() ?: return null
    while (value.tokenType in expressionWrappers) {
        value = significantChildren(source, value)
            .singleOrNull { it.tokenType !in wrapperParts } ?: return null
    }
    return value
}

/**
 * The raw text of the first argument of the call that [source] spans when it is a string template without
 * `$` entries. Escape entries keep their source text, as the Go rules read them; null otherwise.
 */
fun firstArgumentStringContent(source: KtSourceElement): String? {
    val value = firstArgumentNode(source) ?: return null
    if (value.tokenType != KtNodeTypes.STRING_TEMPLATE) return null
    val content = StringBuilder()
    for (part in lightChildren(source, value)) {
        when (part.tokenType) {
            KtTokens.OPEN_QUOTE, KtTokens.CLOSING_QUOTE -> Unit
            KtNodeTypes.LITERAL_STRING_TEMPLATE_ENTRY, KtNodeTypes.ESCAPE_STRING_TEMPLATE_ENTRY ->
                content.append(source.treeStructure.toString(part))
            else -> return null
        }
    }
    return content.toString()
}
