package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.text

// Go matches annotations by simple name, so these helpers do too: a resolved
// annotation uses its class name, an unresolved one the name as written.

/** The simple name of this annotation's class, or the name as written when it does not resolve. */
fun FirAnnotation.simpleName(): String? {
    val type = annotationTypeRef.coneType
    if (type !is ConeErrorType) type.classId?.let { return it.shortClassName.asString() }
    return annotationTypeRef.source?.text?.toString()?.substringAfterLast('.')?.trim()
}

/** True when any annotation on this declaration has one of the given simple [names]. */
fun FirDeclaration.hasAnnotationNamed(names: Set<String>): Boolean =
    annotations.any { it.simpleName() in names }
