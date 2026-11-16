package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.checkers.declaringSourcePath
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedPropertySymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.unwrapFakeOverrides

/**
 * True when the compiler types this operand as nullable.
 *
 * A platform type and a stable smart cast to non-null are not nullable, an unstable smart cast is looked through,
 * and a type parameter with a nullable bound can be null.
 */
context(context: CheckerContext)
fun FirExpression.isNullableOperand(): Boolean {
    var expression = this
    while (expression is FirSmartCastExpression && (!expression.isStable || reliesOnOtherModule(expression))) {
        expression = expression.originalExpression
    }
    return expression.resolvedType.lowerBoundIfFlexible().canBeNull(context.session)
}

// A stable smart cast on a property of another Gradle module is rejected by that module's own build. The check
// compile reads a dependency without build output from source, where the property looks stable, so the module
// directory (the path before `src/<source set>/kotlin`) of every property the cast relies on must be this file's.
context(context: CheckerContext)
private fun reliesOnOtherModule(cast: FirSmartCastExpression): Boolean {
    val own = moduleDirectory(context.containingFile?.path) ?: return false
    var current: FirExpression? = cast.originalExpression
    while (current != null) {
        current = when (current) {
            is FirSmartCastExpression -> current.originalExpression
            is FirPropertyAccessExpression -> {
                val declared = current.calleeReference.toResolvedPropertySymbol()?.unwrapFakeOverrides()?.declaringSourcePath(context.session)
                val module = moduleDirectory(declared)
                if (module != null && module != own) return true
                current.explicitReceiver
            }
            else -> null
        }
    }
    return false
}

private val sourceSetRoot = Regex("/src/[A-Za-z0-9_]+/(?:kotlin|java)/")

private fun moduleDirectory(path: String?): String? {
    val normalized = path?.replace('\\', '/') ?: return null
    val match = sourceSetRoot.findAll(normalized).lastOrNull() ?: return null
    return normalized.substring(0, match.range.first)
}
