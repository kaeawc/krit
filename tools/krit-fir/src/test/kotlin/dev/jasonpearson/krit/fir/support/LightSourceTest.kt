package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertSame
import kotlin.test.assertTrue
import org.jetbrains.kotlin.KtInMemoryTextSourceFile
import org.jetbrains.kotlin.KtLightSourceElement
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.parsing.KotlinLightParser
import dev.jasonpearson.krit.fir.plugins.KtFileParser

class LightSourceTest {
    private val code = "fun greet(name: String) { println(name) }\n"

    // The light-tree parser needs the IntelliJ application that the parse environment sets up.
    private val tree: FlyweightCapableTreeStructure<LighterASTNode> = run {
        KtFileParser.parse(code).close()
        KotlinLightParser.buildLightTree(code, KtInMemoryTextSourceFile("t.kt", "t.kt", code), null)
    }

    private fun sourceFor(node: LighterASTNode, shift: Int = 0): KtSourceElement =
        KtLightSourceElement(node, node.startOffset + shift, node.endOffset + shift, tree, KtRealSourceElementKind)

    private fun find(node: LighterASTNode, type: Any): LighterASTNode? {
        if (node.tokenType == type) return node
        val source = sourceFor(node)
        return lightChildren(source, node).firstNotNullOfOrNull { find(it, type) }
    }

    private fun function(): LighterASTNode = assertNotNull(find(tree.root, KtNodeTypes.FUN))

    @Test
    fun lightChildrenListsDirectChildrenOnly() {
        val root = tree.root
        val kinds = lightChildren(sourceFor(root), root).map { it.tokenType }
        assertTrue(KtNodeTypes.FUN in kinds, "$kinds")
        assertTrue(KtNodeTypes.VALUE_PARAMETER_LIST !in kinds, "nested nodes must not be listed: $kinds")
    }

    @Test
    fun lightChildrenOfALeafIsEmpty() {
        val leaf = lightChildren(sourceFor(function()), function()).first { it.tokenType.toString() == "IDENTIFIER" }
        assertEquals(emptyList(), lightChildren(sourceFor(leaf), leaf))
    }

    @Test
    fun lightSourceOfReturnsTheAnchorForItsOwnNode() {
        val fn = function()
        val anchor = sourceFor(fn, shift = 7)
        assertSame(anchor, lightSourceOf(fn, anchor))
    }

    @Test
    fun lightSourceOfKeepsTheAnchorOffsetShift() {
        val fn = function()
        val anchor = sourceFor(fn, shift = 100)
        val body = assertNotNull(find(fn, KtNodeTypes.BLOCK))
        val source = lightSourceOf(body, anchor)
        assertEquals(body.startOffset + 100, source.startOffset)
        assertEquals(body.endOffset + 100, source.endOffset)
        assertSame(body, source.lighterASTNode)
        assertSame(anchor.treeStructure, source.treeStructure)
    }
}
