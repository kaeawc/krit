package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.builtins.jvm.JavaToKotlinClassMap
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.isCastErased
import org.jetbrains.kotlin.fir.analysis.checkers.isUpcast
import org.jetbrains.kotlin.fir.declarations.utils.isActual
import org.jetbrains.kotlin.fir.declarations.utils.isExpect
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.resolvedType

context(context: CheckerContext)
fun isProvablyImpossibleCast(expression: FirTypeOperatorCall): Boolean {
    val target = expression.conversionTypeRef.coneType.runtimeCastType()
    if (target is ConeErrorType || target.canBeNull(context.session)) return false
    val operand = expression.argumentList.arguments.firstOrNull() ?: return false
    if (isNullLiteral(unwrapSmartCasts(operand))) return expression.operation == FirOperation.AS
    return areFinalCastClassesDisjoint(operand.resolvedType, target)
}

context(context: CheckerContext)
private fun areFinalCastClassesDisjoint(original: ConeKotlinType, target: ConeKotlinType): Boolean {
    if (original is ConeFlexibleType || target is ConeFlexibleType) return false
    val source = original.runtimeCastType()
    if (!source.isConcreteCastType() || !target.isConcreteCastType()) return false
    if (source.hasExpectActualClass() || target.hasExpectActualClass()) return false
    if (!source.isKnownClosedCastClass() || !target.isKnownClosedCastClass()) return false
    return !isCastErased(source, target) && !isUpcast(source, target) && !isUpcast(target, source)
}

context(context: CheckerContext)
private fun ConeKotlinType.runtimeCastType(): ConeKotlinType {
    val expanded = expandedBound()
    val classType = expanded as? ConeClassLikeType ?: return expanded
    val mapped = JavaToKotlinClassMap.mapJavaToKotlin(classType.lookupTag.classId.asSingleFqName()) ?: return expanded
    return mapped.constructClassLikeType(classType.typeArguments, classType.isMarkedNullable)
}

context(context: CheckerContext)
private fun ConeKotlinType.isConcreteCastType(): Boolean =
    this is ConeClassLikeType && this !is ConeErrorType && !isNothingOrNullableNothing && typeArguments.isEmpty()

context(context: CheckerContext)
private fun ConeKotlinType.isKnownClosedCastClass(): Boolean {
    if (this !is ConeClassLikeType || this is ConeErrorType || isNothingOrNullableNothing) return false
    val symbol = lookupTag.toRegularClassSymbol(context.session) ?: return false
    return symbol.classKind in setOf(ClassKind.CLASS, ClassKind.ENUM_CLASS, ClassKind.OBJECT) &&
        symbol.resolvedStatus.modality == Modality.FINAL
}

context(context: CheckerContext)
private fun ConeKotlinType.hasExpectActualClass(): Boolean {
    if (this !is ConeClassLikeType || this is ConeErrorType) return false
    val symbol = lookupTag.toRegularClassSymbol(context.session) ?: return false
    return symbol.resolvedStatus.isExpect || symbol.resolvedStatus.isActual
}
