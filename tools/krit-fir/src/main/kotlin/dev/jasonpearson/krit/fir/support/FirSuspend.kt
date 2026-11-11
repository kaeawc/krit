package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.name.FqName

private val coroutinesPackage = FqName("kotlin.coroutines")

/** Whether [lambda] was resolved against a `suspend` function type. */
fun hasSuspendFunctionType(lambda: FirAnonymousFunction): Boolean {
    val classId = (lambda.typeRef as? FirResolvedTypeRef)?.coneType?.classId ?: return false
    if (classId.packageFqName != coroutinesPackage) return false
    val name = classId.shortClassName.asString()
    return name.startsWith("SuspendFunction") || name.startsWith("KSuspendFunction")
}
