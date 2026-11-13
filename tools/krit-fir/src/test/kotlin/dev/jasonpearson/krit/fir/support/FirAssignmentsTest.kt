package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertNull
import kotlin.test.assertSame
import org.jetbrains.kotlin.fir.FirContractViolation
import org.jetbrains.kotlin.fir.FirExpressionRef
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.builder.buildDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildLiteralExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildVariableAssignment
import org.jetbrains.kotlin.fir.references.builder.buildSimpleNamedReference
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

class FirAssignmentsTest {
    private val access: FirQualifiedAccessExpression = buildPropertyAccessExpression {
        calleeReference = buildSimpleNamedReference { name = Name.identifier("x") }
    }

    private val literal: FirExpression =
        buildLiteralExpression(source = null, kind = ConstantValueKind.Null, value = null, setType = false)

    private fun assignmentTo(target: FirExpression): FirVariableAssignment = buildVariableAssignment {
        lValue = target
        rValue = literal
    }

    @Test
    fun plainAssignmentReturnsItsTargetAccess() {
        assertSame(access, assignedAccess(assignmentTo(access)))
    }

    @OptIn(FirContractViolation::class)
    @Test
    fun compoundAssignmentFollowsTheDesugaredReference() {
        val desugared = buildDesugaredAssignmentValueReferenceExpression {
            expressionRef = FirExpressionRef<FirExpression>().apply { bind(access) }
        }
        assertSame(access, assignedAccess(assignmentTo(desugared)))
    }

    @Test
    fun nonAccessTargetReturnsNull() {
        assertNull(assignedAccess(assignmentTo(literal)))
    }
}
