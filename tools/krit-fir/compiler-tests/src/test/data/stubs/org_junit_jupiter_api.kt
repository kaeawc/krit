// Compiler-test source stubs; never packaged in the production artifact.
package org.junit.jupiter.api

import org.junit.jupiter.api.function.Executable

// Jupiter annotations are Java @Retention(RUNTIME); ANNOTATION_TYPE maps to
// ANNOTATION_CLASS and METHOD to FUNCTION plus the property accessors.

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class Test

@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class Disabled(val value: String = "")

@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class DisplayName(val value: String)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class Nested

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class BeforeEach

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class AfterEach

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class BeforeAll

@Target(
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
@MustBeDocumented
annotation class AfterAll

// Java class of static assertions.
object Assertions {
    fun assertTrue(condition: Boolean) {
        TODO()
    }

    fun assertTrue(condition: Boolean, message: String?) {
        TODO()
    }

    fun assertFalse(condition: Boolean) {
        TODO()
    }

    fun assertEquals(expected: Any?, actual: Any?) {
        TODO()
    }

    fun assertEquals(expected: Any?, actual: Any?, message: String?) {
        TODO()
    }

    fun assertNotNull(actual: Any?) {
        TODO()
    }

    fun assertNotNull(actual: Any?, message: String?) {
        TODO()
    }

    fun assertNull(actual: Any?) {
        TODO()
    }

    fun <T : Throwable> assertThrows(expectedType: Class<T>, executable: Executable): T = TODO()

    fun fail(message: String?): Nothing = TODO()
}

// junit-jupiter-api's Kotlin extensions (AssertionsKt).
inline fun <reified T : Throwable> assertThrows(executable: () -> Unit): T = TODO()

fun assertAll(vararg executables: () -> Unit) {
    TODO()
}
