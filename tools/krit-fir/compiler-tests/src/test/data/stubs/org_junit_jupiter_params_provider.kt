// Compiler-test source stubs; never packaged in the production artifact.
package org.junit.jupiter.params.provider

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class ValueSource(
    val shorts: ShortArray = [],
    val bytes: ByteArray = [],
    val ints: IntArray = [],
    val longs: LongArray = [],
    val floats: FloatArray = [],
    val doubles: DoubleArray = [],
    val chars: CharArray = [],
    val booleans: BooleanArray = [],
    val strings: Array<String> = [],
    val classes: Array<kotlin.reflect.KClass<*>> = [],
)

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class MethodSource(vararg val value: String)

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class CsvSource(vararg val value: String, val delimiter: Char = '\u0000')

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class NullAndEmptySource
