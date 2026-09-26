package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import com.intellij.openapi.util.Ref
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
