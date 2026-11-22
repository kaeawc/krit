package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.declarations.FirResolvedDeclarationStatus
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.text

fun FirAnnotation.resolvedAnnotationClass(session: FirSession): FirRegularClassSymbol? {
    val type = annotationTypeRef.coneType.fullyExpandedType(session)
    if (type is ConeErrorType) return null
    return type.toRegularClassSymbol(session)
}

fun FirAnnotation.writtenAnnotationName(): String? =
    annotationTypeRef.source?.text?.toString()?.substringAfterLast('.')?.trim()?.takeIf { it.isNotEmpty() }

fun FirBasedSymbol<*>.declarationStatus(): FirResolvedDeclarationStatus? = when (this) {
    is FirCallableSymbol<*> -> resolvedStatus
    is FirClassLikeSymbol<*> -> resolvedStatus
    else -> null
}

fun FirBasedSymbol<*>.isEffectivelyPublicApi(): Boolean {
    if (this is FirCallableSymbol<*> && isLocal) return false
    if (this is FirClassLikeSymbol<*> && classId.isLocal) return false
    val status = declarationStatus() ?: return false
    if (status.visibility == Visibilities.Protected) return false
    if (!status.effectiveVisibility.publicApi) return false
    return true
}

fun FirClassLikeSymbol<*>.hasDirectSupertypeAnnotatedWith(marker: ClassId, session: FirSession): Boolean =
    lookupSuperTypes(this, lookupInterfaces = true, deep = false, useSiteSession = session).any { type ->
        type.fullyExpandedType(session).toRegularClassSymbol(session)
            ?.resolvedAnnotationClassIds?.contains(marker) == true
    }
