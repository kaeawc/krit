package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.plugins.KtFileParser
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import org.jetbrains.kotlin.KtInMemoryTextSourceFile
import org.jetbrains.kotlin.KtLightSourceElement
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.parsing.KotlinLightParser

class LightArgumentsTest {
    // The first call expression's (or its qualified parent's) source in `val x = <expression>`.
    private fun callSource(expression: String, qualified: Boolean = false): KtSourceElement {
        val text = "val x = $expression\n"
        KtFileParser.parse(text).close()
        val tree = KotlinLightParser.buildLightTree(text, KtInMemoryTextSourceFile("a.kt", "a.kt", text), null)
        fun find(node: LighterASTNode, type: Any): LighterASTNode? {
            if (node.tokenType == type) return node
            return lightChildren(sourceOf(node, tree), node).firstNotNullOfOrNull { find(it, type) }
        }
        val type = if (qualified) KtNodeTypes.DOT_QUALIFIED_EXPRESSION else KtNodeTypes.CALL_EXPRESSION
        return sourceOf(assertNotNull(find(tree.root, type)), tree)
    }

    private fun sourceOf(
        node: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): KtSourceElement =
        KtLightSourceElement(node, node.startOffset, node.endOffset, tree, KtRealSourceElementKind)

    @Test
    fun readsALiteralFirstArgument() {
        assertEquals("MD5", firstArgumentStringContent(callSource("get(\"MD5\")")))
    }

    @Test
    fun readsThroughAQualifiedCall() {
        assertEquals("MD5", firstArgumentStringContent(callSource("Digest.get(\"MD5\")", qualified = true)))
    }

    @Test
    fun unwrapsParenthesesAnnotationsAndLabels() {
        assertEquals("a", firstArgumentStringContent(callSource("get(((\"a\")))")))
        assertEquals("a", firstArgumentStringContent(callSource("get(@Suppress(\"x\") \"a\")")))
        assertEquals("a", firstArgumentStringContent(callSource("get(l@ \"a\")")))
    }

    @Test
    fun keepsEscapeEntriesAsWritten() {
        assertEquals("a\\tb", firstArgumentStringContent(callSource("get(\"a\\tb\")")))
    }

    @Test
    fun rejectsTemplateEntries() {
        assertNull(firstArgumentStringContent(callSource("get(\"a\$b\")")))
        assertNull(firstArgumentStringContent(callSource("get(\"a\${b}\")")))
    }

    @Test
    fun rejectsNamedSpreadAndNonLiteralArguments() {
        assertNull(firstArgumentStringContent(callSource("get(name = \"a\")")))
        assertNull(firstArgumentStringContent(callSource("get(*arr)")))
        assertNull(firstArgumentStringContent(callSource("get(value)")))
        assertNull(firstArgumentStringContent(callSource("get()")))
    }

    @Test
    fun firstArgumentNodeReturnsTheUnwrappedExpression() {
        val source = callSource("get((7L))")
        val node = assertNotNull(firstArgumentNode(source))
        assertEquals("7L", lightText(source, node))
    }

    @Test
    fun onlyTheFirstArgumentIsRead() {
        assertEquals("a", firstArgumentStringContent(callSource("get(\"a\", \"b\")")))
        assertNull(firstArgumentStringContent(callSource("get(x, \"b\")")))
    }
}
