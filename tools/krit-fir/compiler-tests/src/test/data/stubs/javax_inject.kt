// Compiler-test source stubs; never packaged in the production artifact.
// javax.inject is NOT part of the JDK, so stubbing it does not shadow anything.
package javax.inject

// Java @Target({METHOD, CONSTRUCTOR, FIELD}): no VALUE_PARAMETER and no
// PROPERTY, so `@Inject lateinit var x` lands on the field as in real code.
@MustBeDocumented
@Target(
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
    AnnotationTarget.CONSTRUCTOR,
    AnnotationTarget.FIELD,
)
@Retention(AnnotationRetention.RUNTIME)
annotation class Inject

@MustBeDocumented
@Target(AnnotationTarget.ANNOTATION_CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class Qualifier

@MustBeDocumented
@Target(AnnotationTarget.ANNOTATION_CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class Scope

// No @Target in the Java source: applicable to every declaration.
@Scope
@MustBeDocumented
@Retention(AnnotationRetention.RUNTIME)
annotation class Singleton

@Qualifier
@MustBeDocumented
@Retention(AnnotationRetention.RUNTIME)
annotation class Named(val value: String = "")

interface Provider<T> {
    fun get(): T
}
