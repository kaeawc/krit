package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.renderReadable

/**
 * Render a [ConeKotlinType] as an FQN-qualified type string, matching
 * the krit-types wire format (e.g. `kotlin.String`, `kotlin.Int?`,
 * `kotlin.collections.List<kotlin.String>`). Non-class-like types
 * (type parameters, intersection types, flexible types) fall back to
 * the readable rendering since they don't carry a stable FQN.
 *
 * Extracted so [OracleClassChecker], [OracleExpressionChecker], and
 * any future projection share one renderer — the alternative
 * (per-checker copies) would let backends drift on type spelling
 * across the same compilation.
 */
internal fun ConeKotlinType.renderFqn(): String {
    val classLike = this as? ConeClassLikeType ?: return renderReadable()
    val fqn = classLike.lookupTag.classId.asFqNameString()
    val args = typeArguments
        .takeIf { it.isNotEmpty() }
        ?.joinToString(", ", "<", ">") { projection ->
            (projection as? ConeKotlinTypeProjection)?.type?.renderFqn()
                ?: projection.toString()
        }
        .orEmpty()
    val nullable = if (isMarkedNullable) "?" else ""
    return "$fqn$args$nullable"
}
