package fixtures.positive.style

import kotlin.annotation.AnnotationTarget.FUNCTION
import kotlin.annotation.AnnotationTarget.PROPERTY

@Target(arrayOf(FUNCTION, PROPERTY))
annotation class MyAnnotation
