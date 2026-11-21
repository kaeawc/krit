package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.customAnnotations
import org.jetbrains.kotlin.name.ClassId

/** True when a type annotation resolves to [annotationId], including annotation type aliases. */
context(context: CheckerContext)
fun ConeKotlinType.hasTypeAnnotation(annotationId: ClassId): Boolean =
    customAnnotations.any { it.annotationTypeRef.coneType.expandedBound().classId == annotationId }
