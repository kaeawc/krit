package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.declarations.hasAnnotation
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.arrayElementType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.name.Name

private val MAIN = Name.identifier("main")
private val JVM_STATIC = classIdOf("kotlin.jvm", "JvmStatic")
private val STRING = classIdOf("kotlin", "String")
private val ARRAY = classIdOf("kotlin", "Array")

/**
 * Whether [function], declared directly in [owner], is a program entry point in detekt's sense: a public,
 * non-override `main` taking no parameter, `Array<String>` or `vararg String`, declared at the top level or as a
 * `@JvmStatic` member of an object or companion object. [owner] is the declaration just before [function] in
 * `CheckerContext.containingDeclarations`.
 */
fun isMainFunction(function: FirNamedFunctionSymbol, owner: FirBasedSymbol<*>?, session: FirSession): Boolean {
    if (function.name != MAIN || function.isOverride || function.resolvedStatus.visibility != Visibilities.Public) return false
    if (function.receiverParameterSymbol != null || !hasMainParameter(function, session)) return false
    return when (owner) {
        is FirFileSymbol -> true
        is FirRegularClassSymbol -> owner.classKind == ClassKind.OBJECT && function.hasAnnotation(JVM_STATIC, session)
        else -> false
    }
}

// No parameter, `Array<String>`, or a single `vararg String`.
private fun hasMainParameter(function: FirNamedFunctionSymbol, session: FirSession): Boolean {
    val parameter = function.valueParameterSymbols.let { if (it.isEmpty()) return true else it.singleOrNull() } ?: return false
    val type = parameter.resolvedReturnType.fullyExpandedType(session)
    if (parameter.isVararg) return type.arrayElementType()?.fullyExpandedType(session)?.classId == STRING
    val element = type.typeArguments.singleOrNull() as? ConeKotlinType ?: return false
    return type.classId == ARRAY && element.fullyExpandedType(session).classId == STRING
}
