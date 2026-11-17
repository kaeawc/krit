package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

// Go's "closed later in the same scope" search for a resource held by a named property, read from
// the light tree so comments, strings and lookalike calls (`stmt.useless()`) never count.

private val cleanupNames = setOf("close", "use")
private val accessTokens = setOf(KtTokens.DOT, KtTokens.SAFE_ACCESS)

/** [name] without the backticks it was written with. */
fun unquoteName(name: String): String = name.removeSurrounding("`")

/**
 * Whether the scope of the property whose source is [source] holds `<name>.close(...)` or
 * `<name>.use ...` (safe call or not, receiver optionally qualified, parenthesized or followed by
 * `!!`). The scope is the statements after the property in its block or `when`, or every other
 * member or top-level declaration beside it. [name] is the name as written (backticks allowed).
 */
fun hasCleanupCallInScope(source: KtSourceElement, name: String): Boolean {
    val tree = source.treeStructure
    val node = source.lighterASTNode
    val parent = tree.getParent(node) ?: return false
    val laterOnly = parent.tokenType == KtNodeTypes.BLOCK || parent.tokenType == KtNodeTypes.WHEN
    val wanted = unquoteName(name)
    return lightChildren(source, parent).any { sibling ->
        sibling != node &&
            (!laterOnly || sibling.startOffset >= node.endOffset) &&
            containsCleanup(source, sibling, wanted)
    }
}

private fun containsCleanup(source: KtSourceElement, node: LighterASTNode, name: String): Boolean {
    if (node.tokenType == KtNodeTypes.DOT_QUALIFIED_EXPRESSION ||
        node.tokenType == KtNodeTypes.SAFE_ACCESS_EXPRESSION
    ) {
        if (isCleanupCall(source, node, name)) return true
    }
    return lightChildren(source, node).any { containsCleanup(source, it, name) }
}

private fun isCleanupCall(source: KtSourceElement, node: LighterASTNode, name: String): Boolean {
    val parts = significantChildren(source, node, accessTokens)
    if (parts.size != 2) return false
    val (receiver, selector) = parts
    if (selector.tokenType != KtNodeTypes.CALL_EXPRESSION) return false
    val callee = significantChildren(source, selector).firstOrNull() ?: return false
    if (callee.tokenType != KtNodeTypes.REFERENCE_EXPRESSION) return false
    if (lightText(source, callee) !in cleanupNames) return false
    return receiverName(source, receiver) == name
}

private fun receiverName(source: KtSourceElement, node: LighterASTNode): String? {
    var receiver = unwrapLightParens(source, node)
    while (receiver.tokenType == KtNodeTypes.POSTFIX_EXPRESSION) {
        val parts = significantChildren(source, receiver)
        val operation = parts.getOrNull(1) ?: return null
        if (lightText(source, operation) != "!!") return null
        receiver = unwrapLightParens(source, parts[0])
    }
    if (receiver.tokenType == KtNodeTypes.DOT_QUALIFIED_EXPRESSION ||
        receiver.tokenType == KtNodeTypes.SAFE_ACCESS_EXPRESSION
    ) {
        receiver = significantChildren(source, receiver, accessTokens).lastOrNull() ?: return null
    }
    if (receiver.tokenType != KtNodeTypes.REFERENCE_EXPRESSION) return null
    return unquoteName(lightText(source, receiver))
}
