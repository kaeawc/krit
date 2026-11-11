package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import org.jetbrains.kotlin.fir.expressions.builder.buildLiteralExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildNamedArgumentExpression
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

class FirLiteralsTest {
    @Test
    fun nullLiteralIsRecognised() {
        val literal = buildLiteralExpression(source = null, kind = ConstantValueKind.Null, value = null, setType = false)
        assertTrue(isNullLiteral(literal))
    }

    @Test
    fun otherLiteralsAreNotNull() {
        val literal = buildLiteralExpression(source = null, kind = ConstantValueKind.Int, value = 1, setType = false)
        assertFalse(isNullLiteral(literal))
    }

    @Test
    fun wrappedNullLiteralIsNotUnwrapped() {
        val literal = buildLiteralExpression(source = null, kind = ConstantValueKind.Null, value = null, setType = false)
        val named = buildNamedArgumentExpression {
            expression = literal
            isSpread = false
            name = Name.identifier("value")
        }
        assertFalse(isNullLiteral(named))
    }
}
