package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.checkers.declaringSourcePath
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
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
    while (expression is FirSmartCastExpression && !expression.isStableInCurrentModule()) {
        expression = expression.originalExpression
    }
    return expression.resolvedType.lowerBoundIfFlexible().canBeNull(context.session)
}

context(context: CheckerContext)
fun FirSmartCastExpression.isStableInCurrentModule(): Boolean = isStable && !reliesOnOtherModule(this)

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

/**
 * True when this expression is non-null only through a stable smart cast that relies on a property of another
 * Gradle module (#1672): the cast itself (`a.team.profile` after `a.team.profile != null`), or an implicitly typed
 * local `val` whose inferred type comes from one (`val p = a.team.profile`, then `p?.fields`). The check compiles
 * the scan's modules as one, where such a property is stable; the module's own build rejects the cast, so there
 * the type stays nullable and a redundant-null-safety verdict does not hold.
 */
context(context: CheckerContext)
fun FirExpression.reliesOnCrossModuleSmartCast(): Boolean = reliesOnCrossModuleSmartCast(this, depth = 0)

@OptIn(SymbolInternals::class)
context(context: CheckerContext)
private fun reliesOnCrossModuleSmartCast(expression: FirExpression, depth: Int): Boolean {
    var current: FirExpression? = expression
    while (current != null) {
        current = when (current) {
            is FirSmartCastExpression -> {
                if (current.isStable && reliesOnOtherModule(current)) return true
                current.originalExpression
            }
            is FirPropertyAccessExpression -> {
                val symbol = current.calleeReference.toResolvedPropertySymbol()
                return symbol != null && depth < MAX_LOCAL_INITIALIZER_DEPTH && symbol.isLocal && symbol.isVal &&
                    // An explicit type states the nullability itself, so no smart cast decides it.
                    symbol.fir.returnTypeRef.source?.kind is KtFakeSourceElementKind &&
                    symbol.fir.initializer?.let { initializerReliesOnCrossModuleSmartCast(it, depth + 1) } == true
            }
            else -> null
        }
    }
    return false
}

// Whether the value an initializer produces rests on such a cast. Lambdas are their own scope.
context(context: CheckerContext)
private fun initializerReliesOnCrossModuleSmartCast(initializer: FirExpression, depth: Int): Boolean {
    var found = false
    initializer.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (found || element is FirAnonymousFunctionExpression) return
            if (element is FirExpression && (element is FirSmartCastExpression || element is FirPropertyAccessExpression) &&
                reliesOnCrossModuleSmartCast(element, depth)
            ) {
                found = true
                return
            }
            element.acceptChildren(this)
        }
    })
    return found
}

private const val MAX_LOCAL_INITIALIZER_DEPTH = 3

private val sourceSetRoot = Regex("/src/[A-Za-z0-9_]+/(?:kotlin|java)/")

private fun moduleDirectory(path: String?): String? {
    val normalized = path?.replace('\\', '/') ?: return null
    val match = sourceSetRoot.findAll(normalized).lastOrNull() ?: return null
    return normalized.substring(0, match.range.first)
}
