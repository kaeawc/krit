package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertSame
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildLiteralExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildNamedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildSpreadArgumentExpression
import org.jetbrains.kotlin.name.Name
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
}
