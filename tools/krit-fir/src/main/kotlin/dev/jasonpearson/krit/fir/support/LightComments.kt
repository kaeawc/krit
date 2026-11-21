package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

/** Adjacent preceding comments, following at most one enclosing statement wrapper. */
fun precedingComments(source: KtSourceElement): List<String> {
    var node = source.lighterASTNode
    var hopped = false
    while (true) {
        val parent = source.treeStructure.getParent(node) ?: return emptyList()
        val siblings = lightChildren(source, parent)
        val index = siblings.indexOf(node)
        if (index < 0) return emptyList()
        val comments = ArrayList<String>()
        var hasStatement = false
        for (previous in index - 1 downTo 0) {
            val sibling = siblings[previous]
            when {
                sibling.tokenType == KtTokens.WHITE_SPACE -> Unit
                sibling.tokenType in KtTokens.COMMENTS -> comments += lightText(source, sibling)
                sibling.tokenType == KtTokens.LBRACE -> Unit
                else -> {
                    hasStatement = true
                    break
                }
            }
        }
        if (comments.isNotEmpty() || hasStatement || hopped || parent.tokenType != KtNodeTypes.BLOCK) return comments
        node = parent
        hopped = true
    }
}
