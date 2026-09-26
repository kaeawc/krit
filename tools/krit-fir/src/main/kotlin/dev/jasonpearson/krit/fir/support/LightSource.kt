package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import com.intellij.openapi.util.Ref
import com.intellij.psi.tree.IElementType
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.toKtLightSourceElement

// Light-tree helpers for anchoring a finding on a sub-node of a FIR element's
// source, the way Go reports a specific identifier rather than the whole
// declaration or directive.

/** The direct children of [node], a node in [source]'s tree. */
fun lightChildren(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> {
    val ref = Ref<Array<LighterASTNode?>>()
    source.treeStructure.getChildren(node, ref)
    return ref.get()?.filterNotNull().orEmpty()
}

/**
 * The direct children of [node], a node in [source]'s tree, other than
 * whitespace, comments, and any token type in [skip] (punctuation the caller
 * reads past, such as `.` or parentheses).
 */
fun significantChildren(
    source: KtSourceElement,
    node: LighterASTNode,
    skip: Set<IElementType> = emptySet(),
): List<LighterASTNode> =
    lightChildren(source, node).filter {
        it.tokenType != KtTokens.WHITE_SPACE && it.tokenType !in KtTokens.COMMENTS && it.tokenType !in skip
    }

/**
 * The qualified expression (`r.f()` / `r?.f()`) whose selector is the call
 * whose source is [source]; null when the call has no explicit receiver. K2
 * gives a dot call the whole qualified expression as its source; a safe call
 * keeps the selector call expression, so step up to its parent.
 */
fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
    val node = source.lighterASTNode
    if (node.tokenType in qualifiedExpressionTypes) return node
    if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
    val parent = source.treeStructure.getParent(node) ?: return null
    if (parent.tokenType !in qualifiedExpressionTypes) return null
    val parts = significantChildren(source, parent, accessTokens)
    return parent.takeIf { parts.size > 1 && parts.last() == node }
}

private val qualifiedExpressionTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)
private val accessTokens = setOf(KtTokens.DOT, KtTokens.SAFE_ACCESS)

/** The source text of [node], a node in [source]'s tree, as written. */
fun lightText(source: KtSourceElement, node: LighterASTNode): String =
    source.treeStructure.toString(node).toString()

/**
 * [node] with every enclosing pair of parentheses removed, skipping comments
 * inside them. A pair with nothing inside is returned as it is.
 */
tailrec fun unwrapLightParens(source: KtSourceElement, node: LighterASTNode): LighterASTNode {
    if (node.tokenType != KtNodeTypes.PARENTHESIZED) return node
    val inner = significantChildren(source, node, parenTokens).firstOrNull() ?: return node
    return unwrapLightParens(source, inner)
}

private val parenTokens = setOf(KtTokens.LPAR, KtTokens.RPAR)

/**
 * A source element for [node], a node in [anchor]'s tree, keeping the anchor's
 * shift between tree offsets and file offsets.
 */
fun lightSourceOf(node: LighterASTNode, anchor: KtSourceElement): KtSourceElement {
    if (node == anchor.lighterASTNode) return anchor
    val shift = anchor.startOffset - anchor.lighterASTNode.startOffset
    return node.toKtLightSourceElement(
        anchor.treeStructure,
        startOffset = node.startOffset + shift,
        endOffset = node.endOffset + shift,
    )
}

/**
 * The text of [source]'s first direct IDENTIFIER child as written (backticks
 * included), which is how Go reads a declaration's name; null when absent.
 */
fun identifierText(source: KtSourceElement): String? {
    val identifier = lightChildren(source, source.lighterASTNode)
        .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
    return source.treeStructure.toString(identifier).toString()
}

/**
 * The first annotation or modifier of [declaration]'s direct MODIFIER_LIST
 * child (the modifier list itself when it holds only trivia); null when the
 * declaration has no modifier list. Go reports a declaration on its first
 * line, where the modifier list starts: a preceding KDoc belongs to the
 * declaration in the light tree but not in Go's node, and the first entry
 * stays on that line even when the list spans several.
 */
fun firstModifierAnchor(declaration: KtSourceElement): KtSourceElement? {
    val modifiers = lightChildren(declaration, declaration.lighterASTNode)
        .firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return null
    val first = lightChildren(declaration, modifiers)
        .firstOrNull { it.tokenType !in KtTokens.WHITESPACES && it.tokenType !in KtTokens.COMMENTS }
    return lightSourceOf(first ?: modifiers, declaration)
}

/**
 * The imported name's segment of an import directive: the last selector of its
 * dotted path, which precedes any `as` alias. Go reports that identifier, so
 * `import android.content.Context\n    .MODE_PRIVATE` reports on the name's
 * line, not on the `import` keyword's. Falls back to [directive] when the
 * path is not found.
 */
fun importedNameSource(directive: KtSourceElement): KtSourceElement {
    var node = lightChildren(directive, directive.lighterASTNode).firstOrNull { it.tokenType in importPathTypes }
        ?: return directive
    while (node.tokenType == KtNodeTypes.DOT_QUALIFIED_EXPRESSION) {
        node = lightChildren(directive, node).lastOrNull { it.tokenType in importPathTypes } ?: return directive
    }
    return lightSourceOf(node, directive)
}

private val importPathTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.REFERENCE_EXPRESSION)
