package fixtures.negative.style

import kotlin.annotation.AnnotationTarget.FUNCTION
import kotlin.annotation.AnnotationTarget.PROPERTY

@Target([FUNCTION, PROPERTY])
annotation class MyAnnotation
