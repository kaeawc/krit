package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertEquals
import org.jetbrains.kotlin.KtInMemoryTextSourceFile
import org.jetbrains.kotlin.KtLightSourceElement
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.parsing.KotlinLightParser
import dev.jasonpearson.krit.fir.plugins.KtFileParser

class LightCommentsTest {
    private fun comments(code: String): List<List<String>> {
        KtFileParser.parse(code).close()
        val tree = KotlinLightParser.buildLightTree(code, KtInMemoryTextSourceFile("t.kt", "t.kt", code), null)
        val result = ArrayList<List<String>>()
        fun visit(source: KtSourceElement) {
            if (source.elementType == KtNodeTypes.BINARY_EXPRESSION) result += precedingComments(source)
            lightChildren(source, source.lighterASTNode).forEach { node ->
                visit(KtLightSourceElement(node, node.startOffset, node.endOffset, tree, KtRealSourceElementKind))
            }
        }
        visit(KtLightSourceElement(tree.root, tree.root.startOffset, tree.root.endOffset, tree, KtRealSourceElementKind))
        return result
    }

    @Test
    fun firstStatementAndLaterStatementsKeepOnlyAdjacentComments() {
        assertEquals(
            listOf(listOf("// eager"), emptyList(), listOf("/* safe to repeat */", "// idempotent")),
            comments("fun f() {\n// eager\nx = 1\nx = 2\n// idempotent\n/* safe to repeat */\nx = 3\n}"),
        )
    }

    @Test
    fun functionCommentDoesNotExcuseItsFirstAssignment() {
        assertEquals(listOf(emptyList()), comments("// idempotent\nfun f() { x = 1 }"))
    }
}
