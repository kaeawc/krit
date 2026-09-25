// Compiler-test source stubs; never packaged in the production artifact.
package org.junit

import kotlin.reflect.KClass
import org.junit.function.ThrowingRunnable

// JUnit 4 annotations are Java @Retention(RUNTIME); @Target(METHOD) maps to
// FUNCTION plus the property accessors.

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class Test(val expected: KClass<out Throwable> = Test.None::class, val timeout: Long = 0L) {
    class None private constructor() : Throwable()
}

@Target(
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
    AnnotationTarget.CLASS,
)
@Retention(AnnotationRetention.RUNTIME)
annotation class Ignore(val value: String = "")

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class Before

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class After

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class BeforeClass

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class AfterClass

// Java @Target({FIELD, METHOD}): in Kotlin this forces `@get:Rule` or `@JvmField`.
@Target(
    AnnotationTarget.FIELD,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
annotation class Rule(val order: Int = -1)

@Target(
    AnnotationTarget.FIELD,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
)
@Retention(AnnotationRetention.RUNTIME)
annotation class ClassRule(val order: Int = -1)

// Java class of static assertions (protected constructor).
object Assert {
    fun assertTrue(condition: Boolean) {
        TODO()
    }

    fun assertTrue(message: String?, condition: Boolean) {
        TODO()
    }

    fun assertFalse(condition: Boolean) {
        TODO()
    }

    fun assertFalse(message: String?, condition: Boolean) {
        TODO()
    }

    fun assertEquals(expected: Any?, actual: Any?) {
        TODO()
    }

    fun assertEquals(message: String?, expected: Any?, actual: Any?) {
        TODO()
    }

    fun assertEquals(expected: Long, actual: Long) {
        TODO()
    }

    fun assertEquals(message: String?, expected: Long, actual: Long) {
        TODO()
    }

    fun assertEquals(expected: Double, actual: Double, delta: Double) {
        TODO()
    }

    fun assertNotEquals(unexpected: Any?, actual: Any?) {
        TODO()
    }

    fun assertNull(`object`: Any?) {
        TODO()
    }

    fun assertNotNull(`object`: Any?) {
        TODO()
    }

    fun assertSame(expected: Any?, actual: Any?) {
        TODO()
    }

    fun fail() {
        TODO()
    }

    fun fail(message: String?) {
        TODO()
    }

    fun <T : Throwable> assertThrows(expectedThrowable: Class<T>, runnable: ThrowingRunnable): T = TODO()
}
