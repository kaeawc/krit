// Compiler-test source stubs; never packaged in the production artifact.
package org.junit.jupiter.params

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class ParameterizedTest(val name: String = "[{index}] {argumentsWithNames}")
