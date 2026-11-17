package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.name.ClassId

/**
 * Whether any proper supertype of [symbol] has a class id accepted by [matches].
 *
 * Walks the whole supertype closure through lookup tags, so it is safe for local and anonymous
 * classes. Class ids are never resolved from a symbol that may be local. With [lookupInterfaces]
 * false only superclasses are visited.
 */
inline fun inheritsFrom(
    symbol: FirClassLikeSymbol<*>,
    session: FirSession,
    lookupInterfaces: Boolean,
    matches: (ClassId) -> Boolean,
): Boolean =
    lookupSuperTypes(symbol, lookupInterfaces = lookupInterfaces, deep = true, useSiteSession = session)
        .any { matches(it.lookupTag.classId) }

/** Whether any proper supertype of [symbol] has a class id in [targets]. */
fun inheritsFromAny(
    symbol: FirClassLikeSymbol<*>,
    targets: Set<ClassId>,
    session: FirSession,
    lookupInterfaces: Boolean,
): Boolean = inheritsFrom(symbol, session, lookupInterfaces) { it in targets }

/** Whether [symbol] is [target] or has [target] among its supertypes. */
fun isOrInheritsFrom(
    symbol: FirClassLikeSymbol<*>,
    target: ClassId,
    session: FirSession,
    lookupInterfaces: Boolean,
): Boolean = symbol.classId == target || inheritsFrom(symbol, session, lookupInterfaces) { it == target }
