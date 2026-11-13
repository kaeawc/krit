package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment

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
