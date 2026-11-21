package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWrappedDelegateExpression

/**
 * The variable access that [assignment] writes to, or null when the target is
 * not a qualified access (an array element, for example).
 *
 * `x = y` writes `x` directly. A compound `x += y` that K2 desugars to
 * `x = x.plus(y)` writes through a desugared reference, which is followed to
 * the access it stands for.
 */
fun assignedAccess(assignment: FirVariableAssignment): FirQualifiedAccessExpression? {
    val target = assignment.lValue
    val access = if (target is FirDesugaredAssignmentValueReferenceExpression) target.expressionRef.value else target
    return access as? FirQualifiedAccessExpression
}

/** True when [assignment] is a compound write (`x += y`, `x++`) that K2 desugars through a value reference. */
fun isCompoundAssignment(assignment: FirVariableAssignment): Boolean =
    assignment.lValue is FirDesugaredAssignmentValueReferenceExpression

/** The initializer behind a delegated-property wrapper, or the unchanged expression. */
fun unwrapDelegate(expression: FirExpression): FirExpression =
    if (expression is FirWrappedDelegateExpression) expression.expression else expression
