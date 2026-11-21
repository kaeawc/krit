package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.plugins.KtFileParser
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertSame
import org.jetbrains.kotlin.KtInMemoryTextSourceFile
import org.jetbrains.kotlin.KtLightSourceElement
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildLiteralExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildNamedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildSpreadArgumentExpression
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.parsing.KotlinLightParser
import org.jetbrains.kotlin.types.ConstantValueKind

class FirArgumentsTest {
    private val literal: FirExpression =
        buildLiteralExpression(source = null, kind = ConstantValueKind.Null, value = null, setType = false)

    private fun named(expression: FirExpression): FirExpression =
        buildNamedArgumentExpression {
            this.expression = expression
            isSpread = false
            name = Name.identifier("value")
        }

    private fun spread(expression: FirExpression): FirExpression =
        buildSpreadArgumentExpression { this.expression = expression }

    private fun lambdaExpressionSource(expression: String): KtSourceElement {
        val text = "fun f() { call(onClick = $expression) }\n"
        KtFileParser.parse(text).close()
        val tree = KotlinLightParser.buildLightTree(text, KtInMemoryTextSourceFile("a.kt", "a.kt", text), null)
        fun find(node: LighterASTNode): LighterASTNode? {
            if (node.tokenType == KtNodeTypes.LAMBDA_EXPRESSION) return node
            return lightChildren(sourceOf(node, tree), node).firstNotNullOfOrNull(::find)
        }
        val lambda = requireNotNull(find(tree.root))
        return sourceOf(lambda, tree)
    }

    private fun sourceOf(
        node: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): KtSourceElement =
        KtLightSourceElement(node, node.startOffset, node.endOffset, tree, KtRealSourceElementKind)

    @Test
    fun plainExpressionIsReturnedUnchanged() {
        assertSame(literal, unwrapArgument(literal))
    }

    @Test
    fun namedArgumentIsUnwrapped() {
        assertSame(literal, unwrapArgument(named(literal)))
    }

    @Test
    fun spreadArgumentIsUnwrapped() {
        assertSame(literal, unwrapArgument(spread(literal)))
    }

    @Test
    fun nestedWrappersAreUnwrappedToTheInnermostExpression() {
        assertSame(literal, unwrapArgument(named(spread(literal))))
    }

    @Test
    fun writtenArgumentNameReadsOnlyTheImmediateValueArgumentParent() {
        val source = lambdaExpressionSource("{ }")
        val argument = buildLiteralExpression(source = source, kind = ConstantValueKind.Null, value = null, setType = false)
        assertEquals("onClick", writtenArgumentName(argument))

        for (expression in listOf("({ })", "cb@{ }", "@Suppress(\"unused\") { }", "((cb@{ }))")) {
            val source = lambdaExpressionSource(expression)
            val argument = buildLiteralExpression(source = source, kind = ConstantValueKind.Null, value = null, setType = false)
            assertEquals(null, writtenArgumentName(argument), expression)
        }
    }

    @Test
    fun writtenArgumentNameThroughWrappersReadsLambdaArgumentName() {
        for (expression in listOf("{ }", "({ })", "cb@{ }", "@Suppress(\"unused\") { }", "((cb@{ }))")) {
            val source = lambdaExpressionSource(expression)
            val argument = buildLiteralExpression(source = source, kind = ConstantValueKind.Null, value = null, setType = false)
            assertEquals("onClick", writtenArgumentNameThroughWrappers(argument), expression)
        }
    }
}
